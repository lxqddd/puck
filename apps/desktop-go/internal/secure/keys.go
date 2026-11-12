package secure

import "github.com/zalando/go-keyring"

const service = "puck"

// Keys 把 API Key 存在系统钥匙串里，配置文件只保存供应商信息。
type Keys struct{}

func New() *Keys {
	return &Keys{}
}

func (k *Keys) Set(providerID, key string) error {
	if key == "" {
		return nil
	}
	return keyring.Set(service, providerID, key)
}

func (k *Keys) Get(providerID string) (string, error) {
	return keyring.Get(service, providerID)
}

func (k *Keys) Has(providerID string) bool {
	key, err := keyring.Get(service, providerID)
	return err == nil && key != ""
}
