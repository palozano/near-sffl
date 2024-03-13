package messages

import (
	"github.com/Layr-Labs/eigensdk-go/crypto/bls"

	"github.com/NethermindEth/near-sffl/aggregator/database/models"
	registryrollup "github.com/NethermindEth/near-sffl/contracts/bindings/SFFLRegistryRollup"
	taskmanager "github.com/NethermindEth/near-sffl/contracts/bindings/SFFLTaskManager"
	"github.com/NethermindEth/near-sffl/core"
	coretypes "github.com/NethermindEth/near-sffl/core/types"
)

type MessageBlsAggregation struct {
	EthBlockNumber               coretypes.BlockNumber
	MessageDigest                coretypes.MessageDigest
	NonSignersPubkeysG1          []*bls.G1Point
	QuorumApksG1                 []*bls.G1Point
	SignersApkG2                 *bls.G2Point
	SignersAggSigG1              *bls.Signature
	NonSignerQuorumBitmapIndices []uint32
	QuorumApkIndices             []uint32
	TotalStakeIndices            []uint32
	NonSignerStakeIndices        [][]uint32
}

func NewMessageBlsAggregationFromModel(model models.MessageBlsAggregation) MessageBlsAggregation {
	return MessageBlsAggregation{
		EthBlockNumber:               model.EthBlockNumber,
		MessageDigest:                [32]byte(model.MessageDigest),
		NonSignersPubkeysG1:          model.NonSignersPubkeysG1,
		QuorumApksG1:                 model.QuorumApksG1,
		SignersApkG2:                 model.SignersApkG2,
		SignersAggSigG1:              model.SignersAggSigG1,
		NonSignerQuorumBitmapIndices: model.NonSignerQuorumBitmapIndices,
		QuorumApkIndices:             model.QuorumApkIndices,
		TotalStakeIndices:            model.TotalStakeIndices,
		NonSignerStakeIndices:        model.NonSignerStakeIndices,
	}
}

func (msg MessageBlsAggregation) ToModel() models.MessageBlsAggregation {
	return models.MessageBlsAggregation{
		EthBlockNumber:               msg.EthBlockNumber,
		MessageDigest:                msg.MessageDigest[:],
		NonSignersPubkeysG1:          msg.NonSignersPubkeysG1,
		QuorumApksG1:                 msg.QuorumApksG1,
		SignersApkG2:                 msg.SignersApkG2,
		SignersAggSigG1:              msg.SignersAggSigG1,
		NonSignerQuorumBitmapIndices: msg.NonSignerQuorumBitmapIndices,
		QuorumApkIndices:             msg.QuorumApkIndices,
		TotalStakeIndices:            msg.TotalStakeIndices,
		NonSignerStakeIndices:        msg.NonSignerStakeIndices,
	}
}

func (msg MessageBlsAggregation) ToBindingMainnet() taskmanager.IBLSSignatureCheckerNonSignerStakesAndSignature {
	nonSignersPubkeys := make([]taskmanager.BN254G1Point, 0, len(msg.NonSignersPubkeysG1))
	quorumApks := make([]taskmanager.BN254G1Point, 0, len(msg.QuorumApksG1))

	for _, pubkey := range msg.NonSignersPubkeysG1 {
		nonSignersPubkeys = append(nonSignersPubkeys, core.ConvertToBN254G1Point(pubkey))
	}

	for _, apk := range msg.QuorumApksG1 {
		quorumApks = append(quorumApks, core.ConvertToBN254G1Point(apk))
	}

	return taskmanager.IBLSSignatureCheckerNonSignerStakesAndSignature{
		NonSignerQuorumBitmapIndices: msg.NonSignerQuorumBitmapIndices,
		NonSignerPubkeys:             nonSignersPubkeys,
		QuorumApks:                   quorumApks,
		ApkG2:                        core.ConvertToBN254G2Point(msg.SignersApkG2),
		Sigma:                        core.ConvertToBN254G1Point(msg.SignersAggSigG1.G1Point),
		QuorumApkIndices:             msg.QuorumApkIndices,
		TotalStakeIndices:            msg.TotalStakeIndices,
		NonSignerStakeIndices:        msg.NonSignerStakeIndices,
	}
}

func (msg MessageBlsAggregation) ToBindingRollup() registryrollup.OperatorsSignatureInfo {
	nonSignersPubkeys := make([]registryrollup.BN254G1Point, 0, len(msg.NonSignersPubkeysG1))

	for _, pubkey := range msg.NonSignersPubkeysG1 {
		nonSignersPubkeys = append(nonSignersPubkeys, registryrollup.BN254G1Point(core.ConvertToBN254G1Point(pubkey)))
	}

	return registryrollup.OperatorsSignatureInfo{
		NonSignerPubkeys: nonSignersPubkeys,
		ApkG2:            registryrollup.BN254G2Point(core.ConvertToBN254G2Point(msg.SignersApkG2)),
		Sigma:            registryrollup.BN254G1Point(core.ConvertToBN254G1Point(msg.SignersAggSigG1.G1Point)),
	}
}
