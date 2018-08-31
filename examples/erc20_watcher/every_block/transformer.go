// Copyright 2018 Vulcanize
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package every_block

import (
	"fmt"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/vulcanize/vulcanizedb/examples/generic"
	"github.com/vulcanize/vulcanizedb/libraries/shared"
	"github.com/vulcanize/vulcanizedb/pkg/core"
	"github.com/vulcanize/vulcanizedb/pkg/datastore/postgres"
)

type ERC20Transformer struct {
	Getter     ERC20GetterInterface
	Repository ERC20TokenDatastore
	Retriever  generic.TokenHolderRetriever
	Config     generic.ContractConfig
}

func (t *ERC20Transformer) SetConfiguration(config generic.ContractConfig) {
	t.Config = config
}

type ERC20TokenTransformerInitializer struct {
	Config generic.ContractConfig
}

func (i ERC20TokenTransformerInitializer) NewERC20TokenTransformer(db *postgres.DB, blockchain core.BlockChain) shared.Transformer {
	getter := NewGetter(blockchain)
	repository := ERC20TokenRepository{DB: db}
	retriever := generic.NewTokenHolderRetriever(db, i.Config.Address)
	transformer := ERC20Transformer{
		Getter:     &getter,
		Repository: &repository,
		Retriever:  retriever,
		Config:     i.Config,
	}

	return transformer
}

const (
	FetchingBlocksError         = "Error fetching missing blocks starting at block number %d: %s"
	FetchingSupplyError         = "Error fetching supply for block %d: %s"
	CreateSupplyError           = "Error inserting token_supply for block %d: %s"
	FetchingTokenAddressesError = "Error fetching token holder addresses at block %d: %s"
	FetchingBalanceError        = "Error fetching balance at block %d: %s"
	CreateBalanceError          = "Error inserting token_balance at block %d: %s"
	FetchingAllowanceError      = "Error fetching allowance at block %d: %s"
	CreateAllowanceError        = "Error inserting allowance at block %d: %s"
)

type transformerError struct {
	err         string
	blockNumber int64
	msg         string
}

func (te *transformerError) Error() string {
	return fmt.Sprintf(te.msg, te.blockNumber, te.err)
}

func newTransformerError(err error, blockNumber int64, msg string) error {
	e := transformerError{err.Error(), blockNumber, msg}
	log.Println(e.Error())
	return &e
}

func (t ERC20Transformer) Execute() error {
	var upperBoundBlock int64
	blockchain := t.Getter.GetBlockChain()
	lastBlock := blockchain.LastBlock().Int64()

	if t.Config.LastBlock == -1 {
		upperBoundBlock = lastBlock
	} else {
		upperBoundBlock = t.Config.LastBlock
	}

	// Supply transformations:

	// Fetch missing supply blocks
	blocks, err := t.Repository.MissingSupplyBlocks(t.Config.FirstBlock, upperBoundBlock, t.Config.Address)

	if err != nil {
		return newTransformerError(err, t.Config.FirstBlock, FetchingBlocksError)
	}

	// Fetch supply for missing blocks
	log.Printf("Fetching totalSupply for %d blocks", len(blocks))

	// For each block missing total supply, create supply model and feed the missing data into the repository
	for _, blockNumber := range blocks {
		totalSupply, err := t.Getter.GetTotalSupply(t.Config.Abi, t.Config.Address, blockNumber)

		if err != nil {
			return newTransformerError(err, blockNumber, FetchingSupplyError)
		}
		// Create the supply model
		model := createTokenSupplyModel(totalSupply, t.Config.Address, blockNumber)
		// Feed it into the repository
		err = t.Repository.CreateSupply(model)

		if err != nil {
			return newTransformerError(err, blockNumber, CreateSupplyError)
		}
	}

	// Balance and allowance transformations:

	// Retrieve all token holder addresses for the given contract configuration

	tokenHolderAddresses, err := t.Retriever.RetrieveTokenHolderAddresses()
	if err != nil {
		return newTransformerError(err, t.Config.FirstBlock, FetchingTokenAddressesError)
	}

	// Iterate over the addresses and add their balances and allowances at each block height to the repository
	for holderAddr := range tokenHolderAddresses {

		// Balance transformations:

		blocks, err := t.Repository.MissingBalanceBlocks(t.Config.FirstBlock, upperBoundBlock, t.Config.Address, holderAddr.String())

		if err != nil {
			return newTransformerError(err, t.Config.FirstBlock, FetchingBlocksError)
		}

		log.Printf("Fetching balances for %d blocks", len(blocks))

		// For each block missing balances for the given address, create a balance model and feed the missing data into the repository
		for _, blockNumber := range blocks {

			hashArgs := []common.Address{holderAddr}
			balanceOfArgs := make([]interface{}, len(hashArgs))
			for i, s := range hashArgs {
				balanceOfArgs[i] = s
			}

			totalSupply, err := t.Getter.GetBalance(t.Config.Abi, t.Config.Address, blockNumber, balanceOfArgs)

			if err != nil {
				return newTransformerError(err, blockNumber, FetchingBalanceError)
			}

			model := createTokenBalanceModel(totalSupply, t.Config.Address, blockNumber, holderAddr.String())

			err = t.Repository.CreateBalance(model)

			if err != nil {
				return newTransformerError(err, blockNumber, CreateBalanceError)
			}
		}

		// Allowance transformations:

		for spenderAddr := range tokenHolderAddresses {

			blocks, err := t.Repository.MissingAllowanceBlocks(t.Config.FirstBlock, upperBoundBlock, t.Config.Address, holderAddr.String(), spenderAddr.String())

			if err != nil {
				return newTransformerError(err, t.Config.FirstBlock, FetchingBlocksError)
			}

			log.Printf("Fetching allowances for %d blocks", len(blocks))

			// For each block missing allowances for the given holder and spender addresses, create a allowance model and feed the missing data into the repository
			for _, blockNumber := range blocks {

				hashArgs := []common.Address{holderAddr, spenderAddr}
				allowanceArgs := make([]interface{}, len(hashArgs))
				for i, s := range hashArgs {
					allowanceArgs[i] = s
				}

				totalSupply, err := t.Getter.GetAllowance(t.Config.Abi, t.Config.Address, blockNumber, allowanceArgs)

				if err != nil {
					return newTransformerError(err, blockNumber, FetchingAllowanceError)
				}

				model := createTokenAllowanceModel(totalSupply, t.Config.Address, blockNumber, holderAddr.String(), spenderAddr.String())

				err = t.Repository.CreateAllowance(model)

				if err != nil {
					return newTransformerError(err, blockNumber, CreateAllowanceError)
				}

			}

		}

	}

	return nil
}

func createTokenSupplyModel(totalSupply big.Int, address string, blockNumber int64) TokenSupply {
	return TokenSupply{
		Value:        totalSupply.String(),
		TokenAddress: address,
		BlockNumber:  blockNumber,
	}
}

func createTokenBalanceModel(tokenBalance big.Int, tokenAddress string, blockNumber int64, tokenHolderAddress string) TokenBalance {
	return TokenBalance{
		Value:              tokenBalance.String(),
		TokenAddress:       tokenAddress,
		BlockNumber:        blockNumber,
		TokenHolderAddress: tokenHolderAddress,
	}
}

func createTokenAllowanceModel(tokenBalance big.Int, tokenAddress string, blockNumber int64, tokenHolderAddress, tokenSpenderAddress string) TokenAllowance {
	return TokenAllowance{
		Value:               tokenBalance.String(),
		TokenAddress:        tokenAddress,
		BlockNumber:         blockNumber,
		TokenHolderAddress:  tokenHolderAddress,
		TokenSpenderAddress: tokenSpenderAddress,
	}
}
