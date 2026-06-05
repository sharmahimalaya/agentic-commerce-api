package gateway

type Gateway interface {
	Charge(amount int64, currency string) error
}
