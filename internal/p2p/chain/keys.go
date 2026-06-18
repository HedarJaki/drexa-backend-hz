package chain

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// ethKey bundles a parsed signing key with its derived address.
type ethKey struct {
	priv *ecdsa.PrivateKey
	addr common.Address
}

// parseKey accepts a hex private key with or without a 0x prefix.
func parseKey(hexKey string) (ethKey, error) {
	h := strings.TrimPrefix(strings.TrimSpace(hexKey), "0x")
	priv, err := crypto.HexToECDSA(h)
	if err != nil {
		return ethKey{}, fmt.Errorf("chain: invalid private key: %w", err)
	}
	return ethKey{priv: priv, addr: crypto.PubkeyToAddress(priv.PublicKey)}, nil
}

// IsAddress reports whether s is a syntactically valid EVM (0x…) address.
func IsAddress(s string) bool {
	return common.IsHexAddress(s)
}

// weiPerEth is 10^18, the wei value of 1 ETH.
var weiPerEth = new(big.Float).SetFloat64(1e18)

// EthToWei converts a decimal ETH amount to wei. Intended for demo-scale amounts;
// for exact accounting prefer integer base units end-to-end.
func EthToWei(eth float64) *big.Int {
	f := new(big.Float).SetFloat64(eth)
	f.Mul(f, weiPerEth)
	wei, _ := f.Int(nil)
	return wei
}
