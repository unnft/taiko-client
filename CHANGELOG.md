# Changelog

## [0.7.0](https://github.com/taikoxyz/taiko-client/compare/v0.6.0...v0.7.0) (2023-04-07)


### Features

* **all:** update client softwares based on the new protocol upgrade ([#185](https://github.com/taikoxyz/taiko-client/issues/185)) ([54f7a4c](https://github.com/taikoxyz/taiko-client/commit/54f7a4cb2db72a4ffa9a199e2af1f0d709a1ac27))
* **driver:** changes based on protocol L2 EIP-1559 design ([#188](https://github.com/taikoxyz/taiko-client/issues/188)) ([82e8b97](https://github.com/taikoxyz/taiko-client/commit/82e8b9741782258840696701993b6d009d0260e0))

## [0.6.0](https://github.com/taikoxyz/taiko-client/compare/v0.5.0...v0.6.0) (2023-03-20)


### Features

* **docs:** remove concept docs and refer to website ([#180](https://github.com/taikoxyz/taiko-client/pull/180)) ([a8dcdac](https://github.com/taikoxyz/taiko-client/commit/a8dcdac77c1a5e3f85e4d7a4b912cfb3d903a3d9))
* **flags:** update txpool.locals flag usage ([#181](https://github.com/taikoxyz/taiko-client/pull/181)) ([dac6102](https://github.com/taikoxyz/taiko-client/commit/dac6102d7508b9bdcb248eab4dcf469022353aa8))
* **proposer:** add `proposeEmptyBlockGasLimit` ([#178](https://github.com/taikoxyz/taiko-client/issues/178)) ([e64d769](https://github.com/taikoxyz/taiko-client/commit/e64d769f45d072b151f429f61c1fe2ab07dec0dc))


## [0.5.0](https://github.com/taikoxyz/taiko-client/compare/v0.4.0...v0.5.0) (2023-03-08)


### Features

* **pkg:** improve `BlockBatchIterator` ([#173](https://github.com/taikoxyz/taiko-client/issues/173)) ([4fab06a](https://github.com/taikoxyz/taiko-client/commit/4fab06a9cba17c5e4da09acbe9b95949d6c4d47f))
* **proposer,prover:** make `context.Context` part of `proposer.waitTillSynced` && `ProofProducer.RequestProof`'s parameters ([#169](https://github.com/taikoxyz/taiko-client/issues/169)) ([4b11e29](https://github.com/taikoxyz/taiko-client/commit/4b11e29689b8fac85023669443c351f428a54fea))
* **proposer:** new flag to propose empty blocks ([#175](https://github.com/taikoxyz/taiko-client/issues/175)) ([6621a5c](https://github.com/taikoxyz/taiko-client/commit/6621a5c89a92e7593f702e4c82e69d1215b2ca59))
* **proposer:** remove `poolContentSplitter` in proposer ([#159](https://github.com/taikoxyz/taiko-client/issues/159)) ([e26c831](https://github.com/taikoxyz/taiko-client/commit/e26c831a42fdf448b32bcf75c1f1f87bd71df481))
* **proposer:** remove an unused flag ([#176](https://github.com/taikoxyz/taiko-client/issues/176)) ([7d2126e](https://github.com/taikoxyz/taiko-client/commit/7d2126efe256bcb698b3d4df7352efdbff957ace))
* **prover:** ensure L2 EE is fully synced when calling `initL1Current` ([#170](https://github.com/taikoxyz/taiko-client/issues/170)) ([6c85058](https://github.com/taikoxyz/taiko-client/commit/6c8505827c035cc7456967bc8aab8bec1861e19b))
* **prover:** new flags for `zkevm-chain` ([#166](https://github.com/taikoxyz/taiko-client/issues/166)) ([1c90a3d](https://github.com/taikoxyz/taiko-client/commit/1c90a3d6b7cada0b116875d88f0952993b54bb5f))
* **prover:** tracking for most recent block ID to ensure (relatively) consecutive proving by notification system ([#174](https://github.com/taikoxyz/taiko-client/issues/174)) ([e500039](https://github.com/taikoxyz/taiko-client/commit/e5000395a3a28bd282df64f54867fd771143a56a))


### Bug Fixes

* **proposer:** remove an unused metric from proposer ([#171](https://github.com/taikoxyz/taiko-client/issues/171)) ([8df5eea](https://github.com/taikoxyz/taiko-client/commit/8df5eea1d9f1482a10b7d395ae19953f5d6ea6ce))

## [0.4.0](https://github.com/taikoxyz/taiko-client/compare/v0.3.0...v0.4.0) (2023-02-22)


### Features

* **all:** update contract bindings && some improvements based on Alex's feedback ([#153](https://github.com/taikoxyz/taiko-client/issues/153)) ([bdaa292](https://github.com/taikoxyz/taiko-client/commit/bdaa2920bcb113d3887409edb17462b5e0d3a2c5))
* **bindings:** parse solidity custom errors ([#163](https://github.com/taikoxyz/taiko-client/issues/163)) ([9a79127](https://github.com/taikoxyz/taiko-client/commit/9a79127a5a3cddf4e95ac899943e6551b02cf432))


### Bug Fixes

* **driver:** fix an issue in sync status checking ([#162](https://github.com/taikoxyz/taiko-client/issues/162)) ([4b21027](https://github.com/taikoxyz/taiko-client/commit/4b2102720e2c1c2fcaef1853ad74b91c6d08aaaa))
* **proposer:** fix a proposer nonce order issue ([#157](https://github.com/taikoxyz/taiko-client/issues/157)) ([80fc0e9](https://github.com/taikoxyz/taiko-client/commit/80fc0e94d819f93ecdeac492eb1f35d5f2bb09ce))

## [0.3.0](https://github.com/taikoxyz/taiko-client/compare/v0.2.4...v0.3.0) (2023-02-15)


### Features

* **prover:** improve the check for whether the current block still needs a new proof ([#145](https://github.com/taikoxyz/taiko-client/issues/145)) ([6c00fc5](https://github.com/taikoxyz/taiko-client/commit/6c00fc544b1ed92a4e38860059ef463282648a42))
* **prover:** update `ZkevmRpcdProducer` to make it connecting to a real proverd service ([#121](https://github.com/taikoxyz/taiko-client/issues/121)) ([8c8ee9c](https://github.com/taikoxyz/taiko-client/commit/8c8ee9c2c3266e25e4233821034b89f50bb08c33))
* **repo:** implement release please ([#148](https://github.com/taikoxyz/taiko-client/issues/148)) ([d8f3ad8](https://github.com/taikoxyz/taiko-client/commit/d8f3ad80d358fe547d356b7f7d7fd6e6ca9279ce))
