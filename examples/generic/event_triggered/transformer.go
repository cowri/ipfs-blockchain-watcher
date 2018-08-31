// Copyright 2018 Vulcanize
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

package event_triggered

import (
	"fmt"
	"log"

	"github.com/vulcanize/vulcanizedb/examples/constants"
	"github.com/vulcanize/vulcanizedb/examples/generic"
	"github.com/vulcanize/vulcanizedb/libraries/shared"
	"github.com/vulcanize/vulcanizedb/pkg/datastore"
	"github.com/vulcanize/vulcanizedb/pkg/datastore/postgres"
	"github.com/vulcanize/vulcanizedb/pkg/datastore/postgres/repositories"
)

type GenericTransformer struct {
	Converter              GenericConverterInterface
	WatchedEventRepository datastore.WatchedEventRepository
	FilterRepository       datastore.FilterRepository
	Repository             GenericEventDatastore
}

func NewTransformer(db *postgres.DB, config generic.ContractConfig) (shared.Transformer, error) {
	var transformer shared.Transformer

	cnvtr, err := NewGenericConverter(config)
	if err != nil {
		return transformer, err
	}

	wer := repositories.WatchedEventRepository{DB: db}
	fr := repositories.FilterRepository{DB: db}
	lkr := GenericEventRepository{DB: db}
	transformer = GenericTransformer{
		Converter:              cnvtr,
		WatchedEventRepository: wer,
		FilterRepository:       fr,
		Repository:             lkr,
	}

	for _, filter := range constants.TusdGenericFilters {
		fr.CreateFilter(filter)
	}
	return transformer, nil
}

func (tr GenericTransformer) Execute() error {
	for _, filter := range constants.TusdGenericFilters {
		watchedEvents, err := tr.WatchedEventRepository.GetWatchedEvents(filter.Name)
		if err != nil {
			log.Println(fmt.Sprintf("Error fetching events for %s:", filter.Name), err)
			return err
		}
		for _, we := range watchedEvents {
			if filter.Name == constants.BurnEvent.String() {
				entity, err := tr.Converter.ToBurnEntity(*we)
				model := tr.Converter.ToBurnModel(entity)
				if err != nil {
					log.Printf("Error persisting data for Dai Burns (watchedEvent.LogID %d):\n %s", we.LogID, err)
				}
				tr.Repository.CreateBurn(model, we.LogID)
			}
			if filter.Name == constants.MintEvent.String() {
				entity, err := tr.Converter.ToMintEntity(*we)
				model := tr.Converter.ToMintModel(entity)
				if err != nil {
					log.Printf("Error persisting data for Dai Mints (watchedEvent.LogID %d):\n %s", we.LogID, err)
				}
				tr.Repository.CreateMint(model, we.LogID)
			}
		}
	}
	return nil
}
