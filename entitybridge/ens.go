package entitybridge

import (
	"context"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	contracts "gitlab.com/vocdoni/go-dvote/chain/contracts"
	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"

	log "gitlab.com/vocdoni/go-dvote/log"
)

// ENS wraps the ENS Registry and the ENS Resolver contracts
type ENS struct {
	*contracts.EntityResolver
	*contracts.EnsRegistryWithFallback
	client    *ethclient.Client
	networkID *big.Int
}

func (e *ENS) Init(ctx context.Context, endpoint, registryAddr, resolverAddr string) error {
	var err error
	e.client, err = ethclient.Dial(endpoint)
	if err != nil {
		log.Fatal(err)
	}
	e.networkID, err = e.client.ChainID(ctx)
	if err != nil {
		return err
	}
	log.Infof("found network %s", e.networkID.String())
	if err := e.NewRegistry(registryAddr); err != nil {
		return err
	}
	log.Infof("found registry at address %s", registryAddr)

	if err := e.NewResolver(resolverAddr); err != nil {
		return err
	}
	log.Infof("found resolver at address %s", resolverAddr)
	return nil
}

func (e *ENS) NewRegistry(address string) error {
	var err error
	ethAddress := common.HexToAddress(address)
	e.EnsRegistryWithFallback, err = contracts.NewEnsRegistryWithFallback(ethAddress, e.client)
	if err != nil {
		log.Warnf("error constructing registry contract handle: %s", err)
		return err
	}
	return nil
}

func (e *ENS) NewResolver(address string) error {
	var err error
	ethAddress := common.HexToAddress(address)
	e.EntityResolver, err = contracts.NewEntityResolver(ethAddress, e.client)
	if err != nil {
		log.Warnf("error constructing resolver contract handle: %s", err)
		return err
	}
	return nil
}

func (e *ENS) SetText(signKey *ethereum.SignKeys, node [32]byte, key, value string) error {
	// get nonce
	nonce, err := e.client.PendingNonceAt(context.Background(), signKey.Address())
	if err != nil {
		log.Warnf("error getting signer nonce: %s", err)
		return err
	}
	// use signer key
	auth := bind.NewKeyedTransactor(&signKey.Private)
	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0)             // in wei
	auth.GasLimit = uint64(1000000)        // in units
	auth.GasPrice = big.NewInt(1000000000) // in wei
	// sending tx
	tx, err := e.EntityResolver.SetText(auth, node, key, value)
	if err != nil {
		log.Warnf("error setting resolver text: %s", err)
		return err
	}
	log.Infof("tx sent: %s", tx.Hash().Hex())
	// check text added successfully
	res, err := e.GetText(node, key)
	if err != nil {
		return err
	}
	if res != "" {
		log.Debugf("added text: %s", res)
		return nil
	}
	return errors.New("text was not set, tx failed")
}

func (e *ENS) GetText(node [32]byte, key string) (string, error) {
	text, err := e.EntityResolver.Text(nil, node, key)
	if err != nil {
		log.Warnf("error getting resolver text: %s", err)
		return "", err
	}
	return text, nil
}
