// +build !evm

package polls

import (
	"github.com/loomnetwork/loomchain"
)

type EthSubscriptions struct {
}

func (s EthSubscriptions) AddLogPoll(filter string, height uint64) (string, error) {
	return "", nil
}

func (s EthSubscriptions) AddBlockPoll(height uint64, rpcAddr string) string {
	return ""
}

func (s EthSubscriptions) AddTxPoll(height uint64) string {
	return ""
}

func (s *EthSubscriptions) Poll(state loomchain.ReadOnlyState, id string) ([]byte, error) {
	return nil, nil
}

func (s *EthSubscriptions) Remove(id string) {
}

func NewEthSubscriptions() *EthSubscriptions {
	return &EthSubscriptions{}
}
