// Copyright © 2018 Vulcanize
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"os"

	"time"

	"log"

	"github.com/spf13/cobra"
	"github.com/vulcanize/vulcanizedb/pkg/core"
	"github.com/vulcanize/vulcanizedb/pkg/datastore"
	"github.com/vulcanize/vulcanizedb/pkg/datastore/postgres/repositories"
	"github.com/vulcanize/vulcanizedb/pkg/geth"
	"github.com/vulcanize/vulcanizedb/pkg/history"
	"github.com/vulcanize/vulcanizedb/utils"
)

// syncCmd represents the sync command
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Syncs VulcanizeDB with local ethereum node",
	Long: `Syncs VulcanizeDB with local ethereum node. Populates
Postgres with blocks, transactions, receipts, and logs.

./vulcanizedb sync --starting-block-number 0 --config public.toml

Expects ethereum node to be running and requires a .toml config:

  [database]
  name = "vulcanize_public"
  hostname = "localhost"
  port = 5432

  [client]
  ipcPath = "/Users/user/Library/Ethereum/geth.ipc"
`,
	Run: func(cmd *cobra.Command, args []string) {
		sync()
	},
}

const (
	pollingInterval  = 7 * time.Second
	validationWindow = 15
)

func init() {
	rootCmd.AddCommand(syncCmd)

	syncCmd.Flags().Int64VarP(&startingBlockNumber, "starting-block-number", "s", 0, "Block number to start syncing from")
}

func backFillAllBlocks(blockchain core.Blockchain, blockRepository datastore.BlockRepository, missingBlocksPopulated chan int, startingBlockNumber int64) {
	missingBlocksPopulated <- history.PopulateMissingBlocks(blockchain, blockRepository, startingBlockNumber)
}

func sync() {
	ticker := time.NewTicker(pollingInterval)
	defer ticker.Stop()
	blockchain := geth.NewBlockChain(ipc)

	lastBlock := blockchain.LastBlock().Int64()
	if lastBlock == 0 {
		log.Fatal("geth initial: state sync not finished")
	}
	if startingBlockNumber > lastBlock {
		log.Fatal("starting block number > current block number")
	}

	db := utils.LoadPostgres(databaseConfig, blockchain.Node())
	blockRepository := repositories.NewBlockRepository(&db)
	validator := history.NewBlockValidator(blockchain, blockRepository, validationWindow)
	missingBlocksPopulated := make(chan int)
	go backFillAllBlocks(blockchain, blockRepository, missingBlocksPopulated, startingBlockNumber)

	for {
		select {
		case <-ticker.C:
			window := validator.ValidateBlocks()
			window.Log(os.Stdout)
		case <-missingBlocksPopulated:
			go backFillAllBlocks(blockchain, blockRepository, missingBlocksPopulated, startingBlockNumber)
		}
	}
}
