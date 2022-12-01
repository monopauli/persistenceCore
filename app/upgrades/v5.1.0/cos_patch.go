package v5_1_0

import (
	"encoding/json"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	mintkeeper "github.com/cosmos/cosmos-sdk/x/mint/keeper"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

type CosMints struct {
	Address     string `json:"address"`
	AmountUxprt string `json:"amount"`
}

var (
	cosValidatorAddress = "persistencevaloper1chn6uy6h4zeh5mktapw4cy77getes7fp9hp5pw"
	cosConsensusAddress = "persistencevalcons1a6ga9tuh38nxm56ut0we3t8a8n22cdpdkhh5c8"
)

func mintLostTokens(
	ctx sdk.Context,
	bankKeeper *bankkeeper.BaseKeeper,
	stakingKeeper *stakingkeeper.Keeper,
	mintKeeper *mintkeeper.Keeper,
) {
	var cosMints []CosMints
	err := json.Unmarshal([]byte(recordsJsonString), &cosMints)
	if err != nil {
		panic(fmt.Sprintf("error reading COS JSON: %+v", err))
	}

	cosValAddress, err := sdk.ValAddressFromBech32(cosValidatorAddress)
	if err != nil {
		panic(fmt.Sprintf("validator address is not valid bech32: %s", cosValAddress))
	}

	cosValidator, found := stakingKeeper.GetValidator(ctx, cosValAddress)
	if !found {
		panic(fmt.Sprintf("cos validator '%s' not found", cosValAddress))
	}

	for _, mintRecord := range cosMints {
		coinAmount, mintOk := sdk.NewIntFromString(mintRecord.AmountUxprt)
		if !mintOk {
			panic(fmt.Sprintf("error parsing mint of %suxprt to %s", mintRecord.AmountUxprt, mintRecord.Address))
		}

		coin := sdk.NewCoin("uxprt", coinAmount)
		coins := sdk.NewCoins(coin)

		err = mintKeeper.MintCoins(ctx, coins)
		if err != nil {
			panic(fmt.Sprintf("error minting %suxprt to %s: %+v", mintRecord.AmountUxprt, mintRecord.Address, err))
		}

		delegatorAccount, err := sdk.AccAddressFromBech32(mintRecord.Address)
		if err != nil {
			panic(fmt.Sprintf("error converting human address %s to sdk.AccAddress: %+v", mintRecord.Address, err))
		}

		println("Delegator Account: " + delegatorAccount.String())
		println("Module Name: " + string(minttypes.ModuleName))
		println("Coins: " + coins.String())
		println("Mint Record Amount: " + mintRecord.AmountUxprt)
		println("Mint Record Address: " + mintRecord.Address)
		println("SDK Context: " + string(ctx.BlockHeader().ChainID))

		err = bankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, delegatorAccount, coins)
		if err != nil {
			panic(fmt.Sprintf("error sending minted %suxprt to %s: %+v", mintRecord.AmountUxprt, mintRecord.Address, err))
		}

		sdkAddress, err := sdk.AccAddressFromBech32(mintRecord.Address)
		if err != nil {
			panic(fmt.Sprintf("account address is not valid bech32: %s", mintRecord.Address))
		}

		_, err = stakingKeeper.Delegate(ctx, sdkAddress, coin.Amount, stakingtypes.Unbonded, cosValidator, true)
		if err != nil {
			panic(fmt.Sprintf("error delegating minted %suxprt from %s to %s: %+v", mintRecord.AmountUxprt, mintRecord.Address, cosValidatorAddress, err))
		}
	}
}

func revertTombstone(ctx sdk.Context, slashingKeeper *slashingkeeper.Keeper) error {
	cosValAddress, err := sdk.ValAddressFromBech32(cosValidatorAddress)
	if err != nil {
		panic(fmt.Sprintf("validator address is not valid bech32: %s", cosValAddress))
	}

	cosConsAddress, err := sdk.ConsAddressFromBech32(cosConsensusAddress)
	if err != nil {
		panic(fmt.Sprintf("consensus address is not valid bech32: %s", cosValAddress))
	}

	// Revert Tombstone info
	signInfo, ok := slashingKeeper.GetValidatorSigningInfo(ctx, cosConsAddress)

	if !ok {
		panic(fmt.Sprintf("cannot tombstone validator that does not have any signing information: %s", cosConsAddress.String()))
	}
	if !signInfo.Tombstoned {
		panic(fmt.Sprintf("cannut untombstone a validator that is not tombstoned: %s", cosConsAddress.String()))
	}

	signInfo.Tombstoned = false
	slashingKeeper.SetValidatorSigningInfo(ctx, cosConsAddress, signInfo)
	//slashingKeeper.RevertTombstone(ctx, cosConsAddress)

	// Set jail until=now, the validator then must unjail manually
	slashingKeeper.JailUntil(ctx, cosConsAddress, ctx.BlockTime())

	return nil
}

func RevertCosTombstoning(
	ctx sdk.Context,
	slashingKeeper *slashingkeeper.Keeper,
	mintKeeper *mintkeeper.Keeper,
	bankKeeper *bankkeeper.BaseKeeper,
	stakingKeeper *stakingkeeper.Keeper,
) error {
	err := revertTombstone(ctx, slashingKeeper)
	if err != nil {
		return err
	}

	mintLostTokens(ctx, bankKeeper, stakingKeeper, mintKeeper)

	return nil
}
