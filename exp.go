package main

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/hyperledger-labs/yui-ibc-solidity/pkg/client"
	channeltypes "github.com/hyperledger-labs/yui-ibc-solidity/pkg/ibc/core/channel"
	clienttypes "github.com/hyperledger-labs/yui-ibc-solidity/pkg/ibc/core/client"
	ibctesting "github.com/hyperledger-labs/yui-ibc-solidity/pkg/testing"
	"github.com/hyperledger-labs/yui-ibc-solidity/utils"
)

const (
	relayer          = ibctesting.RelayerKeyIndex // the key-index of relayer on both chains
	deployerA        = ibctesting.RelayerKeyIndex // the key-index of contract deployer on chain A
	deployerB        = ibctesting.RelayerKeyIndex // the key-index of contract deployer on chain B
	aliceA    uint32 = 1                          // the key-index of alice on chain A
	bobB      uint32 = 2                          // the key-index of alice on chain B

	delayPeriodExtensionA = 5
	delayPeriodExtensionB = 10
)

var pair = [][]uint32{{1, 2}, {2, 1}}

// to-do
// 1. transfer token to private key address from alice
// 2. approve token of private key for each pair
// 3. each pair send ibc transfer sequentially
func main() {
	ctx := context.Background()

	ethClA, err := client.NewETHClient("http://127.0.0.1:8645")
	if err != nil {
		fmt.Println(err)
	}
	ethClB, err := client.NewETHClient("http://127.0.0.1:8745")
	if err != nil {
		fmt.Println(err)
	}

	chainA := ibctesting.NewChainWithoutTest(ethClA, ibctesting.NewLightClient(ethClA, clienttypes.BesuIBFT2Client), true)
	// airdropToken(ctx, chainA, ethClA)
	// checkBalance(ctx, chainA)
	// approveToken(ctx, chainA, ethClA, nil)
	// os.Exit(0)
	chainB := ibctesting.NewChainWithoutTest(ethClB, ibctesting.NewLightClient(ethClB, clienttypes.BesuIBFT2Client), true)
	coordinator := ibctesting.NewCoordinatorWithoutTest(chainA, chainB)

	clientA, clientB := coordinator.SetupClientsWithoutTest(ctx, chainA, chainB, clienttypes.BesuIBFT2Client)
	connA, connB := coordinator.CreateConnectionWithoutTest(ctx, chainA, chainB, clientA, clientB)
	chanA, chanB := coordinator.CreateChannelWithoutTest(ctx, chainA, chainB, connA, connB, ibctesting.TransferPort, ibctesting.TransferPort, channeltypes.UNORDERED)
	/// Tests for Transfer module ///
	baseDenom := strings.ToLower(chainA.ContractConfig.ERC20TokenAddress.String())
	expectedDenom := fmt.Sprintf("%v/%v/%v", chanB.PortID, chanB.ID, baseDenom)
	beforeBalanceA, err := chainA.ERC20.BalanceOf(chainA.CallOpts(ctx, relayer), chainA.CallOpts(ctx, deployerA).From)
	fmt.Println("beforeBalanceA:", beforeBalanceA)
	client := 64
	totalTx := 640
	// privateKeyChainA, err := crypto.HexToECDSA(utils.PrivateKeys[0][2:])
	// if err != nil {
	// 	fmt.Println(err)
	// }
	// err = coordinator.ApproveAndDepositToken(ctx, chainA, deployerA, 100, aliceA)

	wg := &sync.WaitGroup{}
	startTime := time.Now()
	wg.Add(1)
	go updateClient(ctx, chainA, chainB, &coordinator, clientA, clientB, chanA, chanB, baseDenom, expectedDenom)
	for i := 0; i < client; i++ {
		wg.Add(1)
		go func(wg *sync.WaitGroup, i uint32) {
			defer wg.Done()
			for j := 0; j < totalTx/client; j++ {
				sendIBCTransfer(ctx, chainA, chainB, &coordinator, clientA, clientB, chanA, chanB, baseDenom, expectedDenom, i)
			}
		}(wg, uint32(i))
	}
	wg.Done()
	wg.Wait()
	fmt.Println("TPS:", float64(totalTx)/time.Since(startTime).Seconds())
	// sendIBCTransfer(ctx, chainA, chainB, &coordinator, clientA, clientB, chanA, chanB, baseDenom, expectedDenom)
	// AfterBalanceB, err := chainB.ICS20Bank.BalanceOf(chainB.CallOpts(ctx, relayer), chainB.CallOpts(ctx, bobB).From, expectedDenom)
	// AfterBalanceA, err := chainA.ERC20.BalanceOf(chainA.CallOpts(ctx, relayer), chainA.CallOpts(ctx, deployerA).From)
	// fmt.Println("AfterBalance A:", AfterBalanceA)
	// fmt.Println("AfterBalance B:", AfterBalanceB)
}

func airdropToken(ctx context.Context, chainA *ibctesting.Chain, client *client.ETHClient) {

	count := 256
	for i := 0; i < count; i++ {
		ecdsaPrivateKey, err := crypto.HexToECDSA(utils.PrivateKeys[i][2:])
		if err != nil {
			fmt.Println(err)
		}
		tx, err := chainA.ERC20.Transfer(chainA.TxOpts(ctx, deployerA), chainA.TxOptsFromPrvKey(ctx, ecdsaPrivateKey).From, big.NewInt(100))
		bind.WaitMined(ctx, client, tx)
	}
}

func approveToken(ctx context.Context, chainA *ibctesting.Chain, client *client.ETHClient, coordinator *ibctesting.Coordinator) {

	count := 256
	wg := &sync.WaitGroup{}

	for i := 0; i < count; i++ {

		wg.Add(1)
		go func(wg *sync.WaitGroup, i int) {
			defer wg.Done()
			ecdsaPrivateKey, err := crypto.HexToECDSA(utils.PrivateKeys[i][2:])
			if err != nil {
				fmt.Println(err)
			}
			fmt.Println("approve token:", i, "for", chainA.TxOptsFromPrvKey(ctx, ecdsaPrivateKey).From)
			err = coordinator.ApproveAndDepositTokenWithPrvKey(ctx, chainA, deployerA, 100, ecdsaPrivateKey)
		}(wg, i)

	}
	wg.Wait()
}

func checkBalance(ctx context.Context, chainA *ibctesting.Chain) {

	count := 256
	for i := 0; i < count; i++ {
		ecdsaPrivateKey, err := crypto.HexToECDSA(utils.PrivateKeys[i][2:])
		if err != nil {
			fmt.Println(err)
		}
		blance, err := chainA.ERC20.BalanceOf(chainA.CallOpts(ctx, relayer), chainA.TxOptsFromPrvKey(ctx, ecdsaPrivateKey).From)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println("address:", chainA.TxOptsFromPrvKey(ctx, ecdsaPrivateKey).From, "balance:", blance)

	}
}

func sendIBCTransfer(ctx context.Context, chainA, chainB *ibctesting.Chain, coordinator *ibctesting.Coordinator, clientA, clientB string, chanA, chanB ibctesting.TestChannel, baseDenom string, expectedDenom string, index uint32) {
	startTime := time.Now()
	privateKeyChainA, err := crypto.HexToECDSA(utils.PrivateKeys[index][2:])
	if err != nil {
		fmt.Println(err)
	}
	privateKeyChainB, err := crypto.HexToECDSA(utils.PrivateKeys[index+120][2:])
	chainA.WaitIfNoError(ctx)(
		chainA.ICS20Transfer.SendTransfer(
			chainA.TxOptsFromPrvKey(ctx, privateKeyChainA),
			baseDenom,
			1,
			chainB.CallOptsFromPrvKey(ctx, privateKeyChainB).From,
			chanA.PortID, chanA.ID,
			uint64(chainB.LastHeader().Number.Int64())+1000,
		),
	)
	// escrowBalance, err := chainA.ICS20Bank.BalanceOf(chainA.CallOpts(ctx, relayer), chainA.ContractConfig.ICS20TransferBankAddress, baseDenom)
	// if err != nil {
	// 	fmt.Println(err)
	// }
	// fmt.Println("escrowBalance", escrowBalance)

	// coordinator.UpdateClient(ctx, chainB, chainA, clientB)
	coordinator.RelayLastSentPacketWithoutTest(ctx, chainA, chainB, chanA, chanB)
	fmt.Println("spent time:", time.Since(startTime))
}

// write a function to continuously update client every 1 seconds
func updateClient(ctx context.Context, chainA, chainB *ibctesting.Chain, coordinator *ibctesting.Coordinator, clientA, clientB string, chanA, chanB ibctesting.TestChannel, baseDenom string, expectedDenom string) {
	for {
		time.Sleep(1 * time.Second)
		coordinator.UpdateClient(ctx, chainB, chainA, clientB)
		fmt.Println("update client")
	}
}
