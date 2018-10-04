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

package vat_init

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/ethereum/go-ethereum/core/types"
)

type Converter interface {
	ToModels(ethLogs []types.Log) ([]VatInitModel, error)
}

type VatInitConverter struct{}

func (VatInitConverter) ToModels(ethLogs []types.Log) ([]VatInitModel, error) {
	var models []VatInitModel
	for _, ethLog := range ethLogs {
		err := verifyLog(ethLog)
		if err != nil {
			return nil, err
		}
		ilk := string(bytes.Trim(ethLog.Topics[1].Bytes(), "\x00"))
		raw, err := json.Marshal(ethLog)
		if err != nil {
			return nil, err
		}
		model := VatInitModel{
			Ilk:              ilk,
			TransactionIndex: ethLog.TxIndex,
			Raw:              raw,
		}
		models = append(models, model)
	}
	return models, nil
}

func verifyLog(log types.Log) error {
	if len(log.Topics) < 2 {
		return errors.New("log missing topics")
	}
	return nil
}
