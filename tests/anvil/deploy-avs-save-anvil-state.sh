#!/bin/bash

RPC_URL=http://localhost:8545
PRIVATE_KEY=0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80

# cd to the directory of this script so that this can be run from anywhere
parent_path=$( cd "$(dirname "${BASH_SOURCE[0]}")" ; pwd -P )
cd "$parent_path"

# start an anvil instance in the background that has eigenlayer contracts deployed
anvil --load-state data/eigenlayer-deployed-anvil-state.json --dump-state data/avs-and-eigenlayer-deployed-anvil-state.json &
cd ../../contracts/evm
forge script script/SFFLDeployer.s.sol --rpc-url $RPC_URL --private-key $PRIVATE_KEY --broadcast -v
# save the block-number in the genesis file which we also need to restart the anvil chain at the correct block
# otherwise the indexRegistry has a quorumUpdate at a high block number, and when we restart a clean anvil (without genesis.json) file
# it starts at block 0, and so calling getOperatorListAtBlockNumber reverts because it thinks there are no quorums registered at block 0
# EDIT: this doesn't actually work... since we can't both load a state and a genesis.json file... see https://github.com/foundry-rs/foundry/issues/6679
# will keep here in case this PR ever gets merged.
GENESIS_FILE=$parent_path/data/genesis.json
TMP_GENESIS_FILE=$parent_path/data/genesis.json.tmp
jq '.number = "'$(cast block-number)'"' $GENESIS_FILE > $TMP_GENESIS_FILE
mv $TMP_GENESIS_FILE $GENESIS_FILE

# we also do this here to make sure the operator has funds to register with the eigenlayer contracts
cast send 0xD5A0359da7B310917d7760385516B2426E86ab7f --value 10ether --private-key 0x2a871d0798f97d79848a013d4936a73bf4cc922c825d33c1cf7073dff6d409c6
# kill anvil to save its state
pkill anvil
