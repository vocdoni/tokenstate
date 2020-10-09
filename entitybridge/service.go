package entitybridge

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/vocdoni/tokenstate/tokenstate"
	"gitlab.com/vocdoni/go-dvote/crypto/ethereum"
	log "gitlab.com/vocdoni/go-dvote/log"
	"gitlab.com/vocdoni/go-dvote/types"
)

type EntityBridgeService struct {
	Token   *tokenstate.Web3
	ENS     *ENS
	SignKey *ethereum.SignKeys
	Gateway string
}

func NewEntityBridgeService() *EntityBridgeService {
	return &EntityBridgeService{
		Token: new(tokenstate.Web3),
		ENS:   new(ENS),
	}
}
func (bs *EntityBridgeService) Init(ctx context.Context, cfg *Config, signKey *ethereum.SignKeys) error {
	bs.SignKey = signKey
	bs.Gateway = cfg.GatewayURL
	// conect to home network and get token contract
	if err := bs.Token.Init(ctx, cfg.Web3HomeEndpoint, cfg.TokenContract); err != nil {
		return err
	}
	// connect foreign network and get ens contracts
	if err := bs.ENS.Init(ctx, cfg.Web3ForeignEndpoint, cfg.RegistryContract, cfg.ResolverContract); err != nil {
		return err
	}
	return nil
}

func (bs *EntityBridgeService) CreateEntityMetadata() (string, error) {
	// get token data
	td, err := bs.Token.GetTokenData()
	if err != nil {
		return "", err
	}
	reqBody, err := json.Marshal(map[string]string{
		"method":  "addFile",
		"type":    "ipfs",
		"name":    "entity-metadata.json",
		"content": base64.StdEncoding.EncodeToString([]byte(td.String())),
	})
	if err != nil {
		return "", err
	}

	// upload entity data to IPFS via gateway
	log.Debug("sending gw request")
	resp, err := http.Post(bs.Gateway,
		"application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var metaResp types.MetaResponse
	if err := json.Unmarshal(body, &metaResp); err != nil {
		return "", nil
	}
	if metaResp.URI == "" {
		return "", fmt.Errorf("response URI cannot be empty")
	}
	log.Debugf("gw response body: %+v", metaResp)
	log.Infof("upload file uri: %s", metaResp.URI)

	// eid == token address
	eIDBytes, err := hex.DecodeString(strings.TrimPrefix(td.Address, "0x"))
	if err != nil {
		return "", err
	}
	var eIDBytes32 [32]byte
	copy(eIDBytes32[:], eIDBytes)

	// set ens record
	if err := bs.ENS.SetText(bs.SignKey, eIDBytes32, "vnd.vocdoni.meta", metaResp.URI); err != nil {
		log.Warnf("cannot set entity metadata: %s", err)
		return "", err
	}
	log.Info("entity created successfully")
	return metaResp.URI, nil
}
