package domain

type Output struct {
	Asset   string
	Value   uint64
	Address string
}

// Withdrawal is used to follow funds withdrawal statistics
type Withdrawal struct {
	TxID              string
	AccountName       string
	Outputs           []Output
	MillisatPerByte   uint64
	TotAmountPerAsset map[string]uint64
	Timestamp         uint64
}
