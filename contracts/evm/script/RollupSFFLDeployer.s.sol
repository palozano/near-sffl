// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.9;

import {Operators} from "../src/rollup/utils/Operators.sol";
import {SFFLRegistryRollup} from "../src/rollup/SFFLRegistryRollup.sol";
import {BN254} from "eigenlayer-middleware/src/libraries/BN254.sol";
import {Utils} from "./utils/Utils.sol";

import "forge-std/Script.sol";
import "forge-std/StdJson.sol";

// forge script script/RollupSFFLDeployer.s.sol:RollupSFFLDeployer --rpc-url $RPC_URL --private-key $PRIVATE_KEY --broadcast -vvvv
contract RollupSFFLDeployer is Script, Utils {
    using BN254 for BN254.G1Point;
    using Operators for Operators.OperatorSet;

    uint128 public constant DEFAULT_WEIGHT = 100;
    uint128 public QUORUM_THRESHOLD = 2 * uint128(100) / 3;

    function run() external {
        vm.startBroadcast();

        BN254.G1Point memory operatorPubkey1 = BN254.G1Point(
            19408553463882111916887171276012224475029133183214861480489485386352635269635,
            17418827901203159022109906145273000034647571131322064812191371351028964064220
        );

        BN254.G1Point memory operatorPubkey2 = BN254.G1Point(
            1611472477336575391907540595283981736749839435478357492584206660415845982634,
            17740534282163859696734712865013083642718796435843138137894885755851743300823
        );

        Operators.Operator[] memory operators = new Operators.Operator[](2);
        operators[0] = Operators.Operator({
            pubkey: operatorPubkey1,
            weight: DEFAULT_WEIGHT
        });
        operators[1] = Operators.Operator({
            pubkey: operatorPubkey2,
            weight: DEFAULT_WEIGHT
        });
        
        uint64 operatorUpdateId = 1;
        SFFLRegistryRollup sfflRegistryRollup = new SFFLRegistryRollup(operators, QUORUM_THRESHOLD, operatorUpdateId);

        {
            string memory parent_object = "parent object";
            string memory deployed_addresses = "addresses";
            string memory deployed_addresses_output = vm.serializeAddress(deployed_addresses, "sfflRegistryRollup", address(sfflRegistryRollup));

            uint256 chainId = block.chainid;
            string memory chain_info = "chainInfo";
            vm.serializeUint(chain_info, "deploymentBlock", block.number);
            string memory chain_info_output = vm.serializeUint(chain_info, "chainId", chainId);

            vm.serializeString(parent_object, deployed_addresses, deployed_addresses_output);
            string memory finalJson = vm.serializeString(parent_object, chain_info, chain_info_output);
            writeOutput(finalJson, "rollup_sffl_deployment_output");
        }

        vm.stopBroadcast();
    }
}