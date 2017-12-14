// Copyright 2017 Monax Industries Limited
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rpc

import (
	"encoding/hex"
	"fmt"
	"strconv"

	acm "github.com/hyperledger/burrow/account"
	"github.com/hyperledger/burrow/client"
	"github.com/hyperledger/burrow/keys"
	"github.com/hyperledger/burrow/logging"
	"github.com/hyperledger/burrow/permission"
	ptypes "github.com/hyperledger/burrow/permission/types"
	"github.com/hyperledger/burrow/txs"
	"github.com/tendermint/go-crypto"
)

//------------------------------------------------------------------------------------
// sign and broadcast convenience

// tx has either one input or we default to the first one (ie for send/bond)
// TODO: better support for multisig and bonding
func signTx(keyClient keys.KeyClient, chainID string, tx_ txs.Tx) (acm.Address, txs.Tx, error) {
	signBytes := acm.SignBytes(chainID, tx_)
	var err error
	switch tx := tx_.(type) {
	case *txs.SendTx:
		signAddress := tx.Inputs[0].Address
		tx.Inputs[0].Signature, err = keyClient.Sign(signAddress, signBytes)
		return signAddress, tx, err

	case *txs.NameTx:
		signAddress := tx.Input.Address
		tx.Input.Signature, err = keyClient.Sign(signAddress, signBytes)
		return signAddress, tx, err

	case *txs.CallTx:
		signAddress := tx.Input.Address
		tx.Input.Signature, err = keyClient.Sign(signAddress, signBytes)
		return signAddress, tx, err

	case *txs.PermissionsTx:
		signAddress := tx.Input.Address
		tx.Input.Signature, err = keyClient.Sign(signAddress, signBytes)
		return signAddress, tx, err

	case *txs.BondTx:
		signAddress := tx.Inputs[0].Address
		tx.Signature, err = keyClient.Sign(signAddress, signBytes)
		tx.Inputs[0].Signature = tx.Signature
		return signAddress, tx, err

	case *txs.UnbondTx:
		signAddress := tx.Address
		tx.Signature, err = keyClient.Sign(signAddress, signBytes)
		return signAddress, tx, err

	case *txs.RebondTx:
		signAddress := tx.Address
		tx.Signature, err = keyClient.Sign(signAddress, signBytes)
		return signAddress, tx, err

	default:
		return acm.ZeroAddress, nil, fmt.Errorf("unknown transaction type for signTx: %#v", tx_)
	}
}

func decodeAddressPermFlag(addrS, permFlagS string) (addr acm.Address, pFlag ptypes.PermFlag, err error) {
	var addrBytes []byte
	if addrBytes, err = hex.DecodeString(addrS); err != nil {
		copy(addr[:], addrBytes)
		return
	}
	if pFlag, err = permission.PermStringToFlag(permFlagS); err != nil {
		return
	}
	return
}

func checkCommon(nodeClient client.NodeClient, keyClient keys.KeyClient, pubkey, addr, amtS,
	nonceS string) (pub acm.PublicKey, amt uint64, nonce uint64, err error) {

	if amtS == "" {
		err = fmt.Errorf("input must specify an amount with the --amt flag")
		return
	}

	if pubkey == "" && addr == "" {
		err = fmt.Errorf("at least one of --pubkey or --addr must be given")
		return
	} else if pubkey != "" {
		if addr != "" {
			logging.InfoMsg(nodeClient.Logger(), "Both a public key and an address have been specified. The public key takes precedent.",
				"public_key", pubkey,
				"address", addr,
			)
		}
		var pubKeyBytes []byte
		pubKeyBytes, err = hex.DecodeString(pubkey)
		if err != nil {
			err = fmt.Errorf("pubkey is bad hex: %v", err)
			return
		}

		pubKeyEd25519 := crypto.PubKeyEd25519{}
		copy(pubKeyEd25519[:], pubKeyBytes)
		pub = acm.PublicKeyFromPubKey(pubKeyEd25519.Wrap())
	} else {
		// grab the pubkey from monax-keys
		addressBytes, err2 := hex.DecodeString(addr)
		if err2 != nil {
			err = fmt.Errorf("Bad hex string for address (%s): %v", addr, err)
			return
		}
		address, err2 := acm.AddressFromBytes(addressBytes)
		if err2 != nil {
			err = fmt.Errorf("Could not convert bytes (%X) to address: %v", addressBytes, err2)
		}
		pub, err2 = keyClient.PublicKey(address)
		if err2 != nil {
			err = fmt.Errorf("Failed to fetch pubkey for address (%s): %v", addr, err2)
			return
		}
	}

	var address acm.Address
	address = pub.Address()

	amt, err = strconv.ParseUint(amtS, 10, 64)
	if err != nil {
		err = fmt.Errorf("amt is misformatted: %v", err)
	}

	if nonceS == "" {
		if nodeClient == nil {
			err = fmt.Errorf("input must specify a nonce with the --nonce flag or use --node-addr (or BURROW_CLIENT_NODE_ADDR) to fetch the nonce from a node")
			return
		}
		// fetch nonce from node
		account, err2 := nodeClient.GetAccount(address)
		if err2 != nil {
			return pub, amt, nonce, err2
		}
		nonce = account.Sequence() + 1
		logging.TraceMsg(nodeClient.Logger(), "Fetch nonce from node",
			"nonce", nonce,
			"account address", address,
		)
	} else {
		nonce, err = strconv.ParseUint(nonceS, 10, 64)
		if err != nil {
			err = fmt.Errorf("nonce is misformatted: %v", err)
			return
		}
	}

	return
}
