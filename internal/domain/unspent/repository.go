package unspent

type Repository interface {
	AddUnspent(unspent []Unspent) error
	GetAllUnspent() []Unspent
	GetBalance(address string, assetHash string) uint64
	GetAvailableUnspent() []Unspent
	GetUnlockedBalance(address string, assetHash string) uint64
}