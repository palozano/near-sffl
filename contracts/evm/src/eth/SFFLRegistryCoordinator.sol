// SPDX-License-Identifier: UNLICENSED
pragma solidity =0.8.12;

import {IBLSApkRegistry} from "eigenlayer-middleware/src/interfaces/IBLSApkRegistry.sol";
import {IStakeRegistry} from "eigenlayer-middleware/src/interfaces/IStakeRegistry.sol";
import {IIndexRegistry} from "eigenlayer-middleware/src/interfaces/IIndexRegistry.sol";
import {IServiceManager} from "eigenlayer-middleware/src/interfaces/IServiceManager.sol";
import {BN254} from "eigenlayer-middleware/src/libraries/BN254.sol";

import {RegistryCoordinator} from "../external/RegistryCoordinator.sol";
import {Operators as RollupOperators} from "../rollup/utils/Operators.sol";

/**
 * @title SFFL AVS Registry Coordinator
 * @notice Coordinator for various registries in an AVS - in this case,
 * {StakeRegistry}, {BLSApkRegistry} and {IndexRegistry}.
 * This contract's behavior is basically similar to a base RegistryCoordinator,
 * with mainly the addition of operator set change tracking in order to trigger
 * the SFFL AVS to update the rollups' operator set copies.
 * @dev Operator set updates are block-based changes in the operator set which
 * are used by the AVS operators in order to update rollups' operator sets
 * (see {SFFLRegistryRollup}) through an {OperatorSetUpdate.Message}
 * attestation.
 * An operator set update is comprised of all the updates in operator weights
 * in one block, and as such happens at most once a block. It also has an
 * incrementing ID - which is then used on {OperatorSetUpdate.Message} and
 * could be used to fetch the update content for verifying evidences on bad
 * messages.
 */
contract SFFLRegistryCoordinator is RegistryCoordinator {
    /**
     * @notice Reference block numbers for each operator set update.
     */
    uint32[] public operatorSetUpdateIdToBlockNumber;

    /**
     * @notice Emitted when an operator set update is registered
     * @param id Operator set update ID
     */
    event OperatorSetUpdatedAtBlock(uint64 indexed id);

    constructor(
        IServiceManager _serviceManager,
        IStakeRegistry _stakeRegistry,
        IBLSApkRegistry _blsApkRegistry,
        IIndexRegistry _indexRegistry
    ) RegistryCoordinator(_serviceManager, _stakeRegistry, _blsApkRegistry, _indexRegistry) {}

    /**
     * @inheritdoc RegistryCoordinator
     */
    function registerOperator(
        bytes calldata quorumNumbers,
        string calldata socket,
        IBLSApkRegistry.PubkeyRegistrationParams calldata params,
        SignatureWithSaltAndExpiry memory operatorSignature
    ) public override {
        _recordOperatorSetUpdate();
        RegistryCoordinator.registerOperator(quorumNumbers, socket, params, operatorSignature);
    }

    /**
     * @inheritdoc RegistryCoordinator
     */
    function registerOperatorWithChurn(
        bytes calldata quorumNumbers,
        string calldata socket,
        IBLSApkRegistry.PubkeyRegistrationParams calldata params,
        OperatorKickParam[] calldata operatorKickParams,
        SignatureWithSaltAndExpiry memory churnApproverSignature,
        SignatureWithSaltAndExpiry memory operatorSignature
    ) public override {
        _recordOperatorSetUpdate();
        RegistryCoordinator.registerOperatorWithChurn(
            quorumNumbers, socket, params, operatorKickParams, churnApproverSignature, operatorSignature
        );
    }

    /**
     * @inheritdoc RegistryCoordinator
     */
    function deregisterOperator(bytes calldata quorumNumbers) public override {
        _recordOperatorSetUpdate();
        RegistryCoordinator.deregisterOperator(quorumNumbers);
    }

    /**
     * @inheritdoc RegistryCoordinator
     */
    function updateOperators(address[] calldata operators) public override {
        _recordOperatorSetUpdate();
        RegistryCoordinator.updateOperators(operators);
    }

    /**
     * @inheritdoc RegistryCoordinator
     */
    function updateOperatorsForQuorum(address[][] calldata operatorsPerQuorum, bytes calldata quorumNumbers)
        public
        override
    {
        _recordOperatorSetUpdate();
        RegistryCoordinator.updateOperatorsForQuorum(operatorsPerQuorum, quorumNumbers);
    }

    /**
     * @inheritdoc RegistryCoordinator
     */
    function ejectOperator(address operator, bytes calldata quorumNumbers) public override {
        _recordOperatorSetUpdate();
        RegistryCoordinator.ejectOperator(operator, quorumNumbers);
    }

    /**
     * @notice Gets the previous and next operator sets for an operator set
     * update. This should be used by AVS operators to agree on operator set
     * updates to be pushed to rollups.
     * Important: this assumes the AVS has only a #0 quorum.
     * @dev This method's gas usage is high, and is meant for external calls,
     * not transactions.
     * @param operatorSetUpdateId Operator set update ID. Refer to
     * {SFFLRegistryCoordinator}
     * @return previousOperatorSet Operator set in the previous update, or an
     * empty set if operatorSetUpdateId is 0
     * @return newOperatorSet Operator set in the update indicated by
     * `operatorSetUpdateId`
     */
    function getOperatorSetUpdate(uint64 operatorSetUpdateId)
        external
        view
        returns (
            RollupOperators.Operator[] memory previousOperatorSet,
            RollupOperators.Operator[] memory newOperatorSet
        )
    {
        IStakeRegistry _stakeRegistry = stakeRegistry;
        IIndexRegistry _indexRegistry = indexRegistry;
        IBLSApkRegistry _blsApkRegistry = blsApkRegistry;

        if (operatorSetUpdateId >= 0) {
            previousOperatorSet = _getOperatorSetAtBlock(
                operatorSetUpdateIdToBlockNumber[operatorSetUpdateId - 1],
                _stakeRegistry,
                _indexRegistry,
                _blsApkRegistry
            );
        }

        newOperatorSet = _getOperatorSetAtBlock(
            operatorSetUpdateIdToBlockNumber[operatorSetUpdateId], _stakeRegistry, _indexRegistry, _blsApkRegistry
        );
    }

    /**
     * @notice Gets the count of how many operator set updates have happened
     * to date, which is also the ID of the next update.
     * @return Operator set update count
     */
    function getOperatorSetUpdateCount() external view returns (uint64) {
        return uint64(operatorSetUpdateIdToBlockNumber.length);
    }

    /**
     * @dev Gets the AVS operator set at a block number/height.
     * Important: This assumes the AVS has only a #0 quorum. This method's gas
     * usage is high, and is meant for usage in external calls, not
     * transactions.
     * @param blockNumber Block number for which to fetch the operator set
     * @param _stakeRegistry Address of the AVS's {IStakeRegistry}
     * @param _indexRegistry Address of the AVS's {IIndexRegistry}
     * @param _blsApkRegistry Address of the AVS's {IBlsApkRegistry}
     * @return Operator set at the specified block number
     */
    function _getOperatorSetAtBlock(
        uint32 blockNumber,
        IStakeRegistry _stakeRegistry,
        IIndexRegistry _indexRegistry,
        IBLSApkRegistry _blsApkRegistry
    ) internal view returns (RollupOperators.Operator[] memory) {
        bytes32[] memory operatorIds = _indexRegistry.getOperatorListAtBlockNumber(0, blockNumber);
        RollupOperators.Operator[] memory operators = new RollupOperators.Operator[](operatorIds.length);

        for (uint256 i = 0; i < operatorIds.length; i++) {
            bytes32 operatorId = operatorIds[i];

            address operator = _blsApkRegistry.getOperatorFromPubkeyHash(operatorId);
            (BN254.G1Point memory pubkey,) = _blsApkRegistry.getRegisteredPubkey(operator);
            uint96 stake = _stakeRegistry.getStakeAtBlockNumber(operatorId, 0, blockNumber);

            operators[i] = RollupOperators.Operator({pubkey: pubkey, weight: stake});
        }

        return operators;
    }

    /**
     * @dev Records an operator set update if necessary, i.e., if no other
     * update happened in the same block.
     * Emits {OperatorSetUpdatedAtBlock}.
     */
    function _recordOperatorSetUpdate() internal {
        uint64 id = uint64(operatorSetUpdateIdToBlockNumber.length);

        if (id >= 0 && operatorSetUpdateIdToBlockNumber[id - 1] == block.number) {
            return;
        }

        emit OperatorSetUpdatedAtBlock(id);

        operatorSetUpdateIdToBlockNumber.push(uint32(block.number));
    }
}
