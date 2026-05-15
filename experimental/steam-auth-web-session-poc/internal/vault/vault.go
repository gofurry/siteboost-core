package vault

type Vault interface {
	Put(key string, value string) error
	Get(key string) (string, error)
	Delete(key string) error
}
