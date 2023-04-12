package prover

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/taikoxyz/taiko-client/bindings"
	"github.com/taikoxyz/taiko-client/metrics"
	eventIterator "github.com/taikoxyz/taiko-client/pkg/chain_iterator/event_iterator"
	"github.com/taikoxyz/taiko-client/pkg/rpc"
	txListValidator "github.com/taikoxyz/taiko-client/pkg/tx_list_validator"
	proofProducer "github.com/taikoxyz/taiko-client/prover/proof_producer"
	proofSubmitter "github.com/taikoxyz/taiko-client/prover/proof_submitter"
	"github.com/urfave/cli/v2"
)

// Prover keep trying to prove new proposed blocks valid/invalid.
type Prover struct {
	// Configurations
	cfg           *Config
	proverAddress common.Address

	// Clients
	rpc *rpc.Client

	// Contract configurations
	txListValidator *txListValidator.TxListValidator
	protocolConfigs *bindings.TaikoDataConfig

	// States
	latestVerifiedL1Height uint64
	lastHandledBlockID     uint64
	l1Current              uint64

	// Proof submitters
	validProofSubmitter   proofSubmitter.ProofSubmitter
	invalidProofSubmitter proofSubmitter.ProofSubmitter

	// Subscriptions
	blockProposedCh  chan *bindings.TaikoL1ClientBlockProposed
	blockProposedSub event.Subscription
	blockVerifiedCh  chan *bindings.TaikoL1ClientBlockVerified
	blockVerifiedSub event.Subscription
	proveNotify      chan struct{}

	// Proof related
	proveValidProofCh   chan *proofProducer.ProofWithHeader
	proveInvalidProofCh chan *proofProducer.ProofWithHeader

	// Concurrency guards
	proposeConcurrencyGuard     chan struct{}
	submitProofConcurrencyGuard chan struct{}
	submitProofTxMutex          *sync.Mutex

	ctx context.Context
	wg  sync.WaitGroup
}

// New initializes the given prover instance based on the command line flags.
func (p *Prover) InitFromCli(ctx context.Context, c *cli.Context) error {
	cfg, err := NewConfigFromCliContext(c)
	if err != nil {
		return err
	}

	return InitFromConfig(ctx, p, cfg)
}

// InitFromConfig initializes the prover instance based on the given configurations.
func InitFromConfig(ctx context.Context, p *Prover, cfg *Config) (err error) {
	p.cfg = cfg
	p.ctx = ctx

	// Clients
	if p.rpc, err = rpc.NewClient(p.ctx, &rpc.ClientConfig{
		L1Endpoint:     cfg.L1WsEndpoint,
		L2Endpoint:     cfg.L2WsEndpoint,
		TaikoL1Address: cfg.TaikoL1Address,
		TaikoL2Address: cfg.TaikoL2Address,
	}); err != nil {
		return err
	}

	// Configs
	protocolConfigs, err := p.rpc.TaikoL1.GetConfig(nil)
	if err != nil {
		return fmt.Errorf("failed to get protocol configs: %w", err)
	}
	p.protocolConfigs = &protocolConfigs

	log.Info("Protocol configs", "configs", p.protocolConfigs)

	p.submitProofTxMutex = &sync.Mutex{}
	p.txListValidator = txListValidator.NewTxListValidator(
		p.protocolConfigs.BlockMaxGasLimit.Uint64(),
		p.protocolConfigs.MaxTransactionsPerBlock.Uint64(),
		p.protocolConfigs.MaxBytesPerTxList.Uint64(),
		p.protocolConfigs.MinTxGasLimit.Uint64(),
		p.rpc.L2ChainID,
	)
	p.proverAddress = crypto.PubkeyToAddress(p.cfg.L1ProverPrivKey.PublicKey)

	chBufferSize := 204800
	p.blockProposedCh = make(chan *bindings.TaikoL1ClientBlockProposed, chBufferSize)
	p.blockVerifiedCh = make(chan *bindings.TaikoL1ClientBlockVerified, chBufferSize)
	p.proveValidProofCh = make(chan *proofProducer.ProofWithHeader, chBufferSize)
	p.proveInvalidProofCh = make(chan *proofProducer.ProofWithHeader, chBufferSize)
	p.proveNotify = make(chan struct{}, 1)

	backoff.Retry(func() error {
		if ctx.Err() != nil {
			return nil
		}
		return p.initL1Current(cfg.StartingBlockID)
	}, backoff.NewExponentialBackOff())

	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Concurrency guards
	p.proposeConcurrencyGuard = make(chan struct{}, cfg.MaxConcurrentProvingJobs)
	p.submitProofConcurrencyGuard = make(chan struct{}, cfg.MaxConcurrentProvingJobs)

	var producer proofProducer.ProofProducer
	if cfg.Dummy {
		producer = &proofProducer.DummyProofProducer{
			RandomDummyProofDelayLowerBound: p.cfg.RandomDummyProofDelayLowerBound,
			RandomDummyProofDelayUpperBound: p.cfg.RandomDummyProofDelayUpperBound,
		}
	} else {
		if producer, err = proofProducer.NewZkevmRpcdProducer(
			cfg.ZKEvmRpcdEndpoint,
			cfg.ZkEvmRpcdParamsPath,
			cfg.L1HttpEndpoint,
			cfg.L2HttpEndpoint,
			true,
		); err != nil {
			return err
		}
	}

	// Proof submitters
	p.validProofSubmitter = proofSubmitter.NewValidProofSubmitter(
		p.rpc,
		producer,
		p.proveValidProofCh,
		p.cfg.TaikoL2Address,
		p.cfg.L1ProverPrivKey,
		p.cfg.ProofSubmittorPrivKey,
		p.submitProofTxMutex,
	)
	p.invalidProofSubmitter = proofSubmitter.NewInvalidProofSubmitter(
		p.rpc,
		producer,
		p.proveInvalidProofCh,
		p.cfg.L1ProverPrivKey,
		protocolConfigs.AnchorTxGasLimit.Uint64(),
		p.submitProofTxMutex,
	)

	return nil
}

// Start starts the main loop of the L2 block prover.
func (p *Prover) Start() error {
	p.wg.Add(1)
	p.initSubscription()
	go p.eventLoop()

	return nil
}

// eventLoop starts the main loop of Taiko prover.
func (p *Prover) eventLoop() {
	defer func() {
		p.wg.Done()
	}()

	// reqProving requests performing a proving operation, won't block
	// if we are already proving.
	reqProving := func() {
		select {
		case p.proveNotify <- struct{}{}:
		default:
		}
	}

	// If there is too many (TaikoData.Config.maxNumBlocks) pending blocks in TaikoL1 contract, there will be no new
	// BlockProposed temporarily, so except the BlockProposed subscription, we need another trigger to start
	// fetching the proposed blocks.
	forceProvingTicker := time.NewTicker(15 * time.Second)
	defer forceProvingTicker.Stop()

	// Call reqProving() right away to catch up with the latest state.
	reqProving()

	go func() {
		for {
			select {
			case <-p.ctx.Done():
				return
			case <-time.NewTicker(30 * time.Second).C:
				vars, err := p.rpc.GetProtocolStateVariables(nil)
				if err != nil {
					log.Error("Get protocol state variables error", "error", err)
					continue
				}

				metrics.ProverPendingBlocksGauge.Update(int64(vars.NextBlockId - vars.LatestVerifiedId - 1))
			}
		}
	}()

	for {
		select {
		case <-p.ctx.Done():
			return
		case proofWithHeader := <-p.proveValidProofCh:
			p.submitProofOp(p.ctx, proofWithHeader, true)
		case proofWithHeader := <-p.proveInvalidProofCh:
			p.submitProofOp(p.ctx, proofWithHeader, false)
		case <-p.proveNotify:
			if err := p.proveOp(); err != nil {
				log.Error("Prove new blocks error", "error", err)
			}
		case <-p.blockProposedCh:
			reqProving()
		case e := <-p.blockVerifiedCh:
			if err := p.onBlockVerified(p.ctx, e); err != nil {
				log.Error("Handle BlockVerified event error", "error", err)
			}
		case <-forceProvingTicker.C:
			reqProving()
		}
	}
}

// Close closes the prover instance.
func (p *Prover) Close() {
	p.closeSubscription()
	p.wg.Wait()
}

// proveOp performs a proving operation, find current unproven blocks, then
// request generating proofs for them.
func (p *Prover) proveOp() error {
	iter, err := eventIterator.NewBlockProposedIterator(p.ctx, &eventIterator.BlockProposedIteratorConfig{
		Client:               p.rpc.L1,
		TaikoL1:              p.rpc.TaikoL1,
		StartHeight:          new(big.Int).SetUint64(p.l1Current),
		OnBlockProposedEvent: p.onBlockProposed,
	})
	if err != nil {
		return err
	}

	return iter.Iter()
}

// onBlockProposed tries to prove that the newly proposed block is valid/invalid.
func (p *Prover) onBlockProposed(
	ctx context.Context,
	event *bindings.TaikoL1ClientBlockProposed,
	end eventIterator.EndBlockProposedEventIterFunc,
) error {
	// If there is newly generated proofs, we need to submit them as soon as possible.
	if len(p.proveValidProofCh) > 0 || len(p.proveInvalidProofCh) > 0 {
		end()
		return nil
	}
	if event.Id.Uint64() <= p.lastHandledBlockID {
		return nil
	}
	if p.cfg.OnlyProveEvenNumberBlocks && event.Id.Uint64()%2 != 0 {
		log.Info("Skip a block with odd number", "id", event.Id)
		return nil
	}
	if p.cfg.OnlyProveOddNumberBlocks && event.Id.Uint64()%2 == 0 {
		log.Info("Skip a block with even number", "id", event.Id)
		return nil
	}

	log.Info("Proposed block", "blockID", event.Id)
	metrics.ProverReceivedProposedBlockGauge.Update(event.Id.Int64())

	handleBlockProposedEvent := func() error {
		defer func() { <-p.proposeConcurrencyGuard }()

		// Check whether the block has been verified.
		isVerified, err := p.isBlockVerified(event.Id)
		if err != nil {
			return err
		}

		if isVerified {
			log.Info("📋 Block has been verified", "blockID", event.Id)
			return nil
		}

		needNewProof, err := p.NeedNewProof(event.Id)
		if err != nil {
			return fmt.Errorf("failed to check whether the L2 block needs a new proof: %w", err)
		}

		if !needNewProof {
			return nil
		}

		// Check whether the transactions list is valid.
		proposeBlockTx, err := p.rpc.L1.TransactionInBlock(ctx, event.Raw.BlockHash, event.Raw.TxIndex)
		if err != nil {
			return err
		}

		_, hint, _, err := p.txListValidator.ValidateTxList(event.Id, proposeBlockTx.Data())
		if err != nil {
			return err
		}

		// Prove the proposed block is valid.
		if hint == txListValidator.HintOK {
			return p.validProofSubmitter.RequestProof(ctx, event)
		}

		// Otherwise, prove the proposed block is invalid.
		return p.invalidProofSubmitter.RequestProof(ctx, event)
	}

	p.proposeConcurrencyGuard <- struct{}{}

	p.l1Current = event.Raw.BlockNumber
	p.lastHandledBlockID = event.Id.Uint64()

	go func() {
		if err := handleBlockProposedEvent(); err != nil {
			log.Error("Handle new BlockProposed event error", "error", err)
		}
	}()

	return nil
}

// submitProofOp performs a (valid block / invalid block) proof submission operation.
func (p *Prover) submitProofOp(ctx context.Context, proofWithHeader *proofProducer.ProofWithHeader, isValidProof bool) {
	p.submitProofConcurrencyGuard <- struct{}{}
	go func() {
		defer func() { <-p.submitProofConcurrencyGuard }()

		var err error
		if isValidProof {
			// If its the oracle prover, will keep retrying when there are errors.
			if p.cfg.Dummy {
				err = backoff.Retry(func() error {
					if err := p.validProofSubmitter.SubmitProof(p.ctx, proofWithHeader, true); err != nil {
						log.Info("Retry oracle proving", "error", err)
						return err
					}

					return nil
				}, backoff.NewConstantBackOff(12*time.Second))
			} else {
				err = p.validProofSubmitter.SubmitProof(p.ctx, proofWithHeader, false)
			}
		} else {
			err = p.invalidProofSubmitter.SubmitProof(p.ctx, proofWithHeader, false)
		}

		if err != nil {
			log.Error("Submit proof error", "isValidProof", isValidProof, "error", err)
		}
	}()
}

// onBlockVerified update the latestVerified block in current state.
// TODO: cancel the corresponding block's proof generation, if requested before.
func (p *Prover) onBlockVerified(ctx context.Context, event *bindings.TaikoL1ClientBlockVerified) error {
	metrics.ProverLatestVerifiedIDGauge.Update(event.Id.Int64())
	p.latestVerifiedL1Height = event.Raw.BlockNumber

	if event.BlockHash == (common.Hash{}) {
		log.Info("New verified invalid block", "blockID", event.Id)
		return nil
	}

	log.Info("New verified valid block", "blockID", event.Id, "hash", common.BytesToHash(event.BlockHash[:]))
	return nil
}

// Name returns the application name.
func (p *Prover) Name() string {
	return "prover"
}

// initL1Current initializes prover's L1Current cursor.
func (p *Prover) initL1Current(startingBlockID *big.Int) error {
	if err := p.rpc.WaitTillL2Synced(p.ctx); err != nil {
		return err
	}

	if startingBlockID == nil {
		stateVars, err := p.rpc.GetProtocolStateVariables(nil)
		if err != nil {
			return err
		}

		if stateVars.LatestVerifiedId == 0 {
			p.l1Current = stateVars.GenesisHeight
			return nil
		}

		startingBlockID = new(big.Int).SetUint64(stateVars.LatestVerifiedId)
	}

	latestVerifiedHeaderL1Origin, err := p.rpc.L2.L1OriginByID(p.ctx, startingBlockID)
	if err != nil {
		return err
	}

	p.l1Current = latestVerifiedHeaderL1Origin.L1BlockHeight.Uint64()
	return nil
}

// isBlockVerified checks whether the given block has been verified by other provers.
func (p *Prover) isBlockVerified(id *big.Int) (bool, error) {
	stateVars, err := p.rpc.GetProtocolStateVariables(nil)
	if err != nil {
		return false, err
	}

	return id.Uint64() <= stateVars.LatestVerifiedId, nil
}

// NeedNewProof checks whether the L2 block still needs a new proof.
func (p *Prover) NeedNewProof(id *big.Int) (bool, error) {
	var parentHash common.Hash
	if id.Cmp(common.Big1) == 0 {
		header, err := p.rpc.L2.HeaderByNumber(p.ctx, common.Big0)
		if err != nil {
			return false, err
		}

		parentHash = header.Hash()
	} else {
		parentL1Origin, err := p.rpc.WaitL1Origin(p.ctx, new(big.Int).Sub(id, common.Big1))
		if err != nil {
			return false, err
		}

		parentHash = parentL1Origin.L2BlockHash
	}

	fc, err := p.rpc.TaikoL1.GetForkChoice(nil, id, parentHash)
	if err != nil {
		return false, err
	}

	if fc.Prover != (common.Address{}) {
		log.Info("📬 Block's proof has already been submitted", "blockID", id, "prover", fc.Prover)
		return false, nil
	}

	return true, nil
}

// initSubscription initializes all subscriptions in current prover instance.
func (p *Prover) initSubscription() {
	p.blockProposedSub = rpc.SubscribeBlockProposed(p.rpc.TaikoL1, p.blockProposedCh)
	p.blockVerifiedSub = rpc.SubscribeBlockVerified(p.rpc.TaikoL1, p.blockVerifiedCh)
}

// closeSubscription closes all subscriptions.
func (p *Prover) closeSubscription() {
	p.blockVerifiedSub.Unsubscribe()
	p.blockProposedSub.Unsubscribe()
}
