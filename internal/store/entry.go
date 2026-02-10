package store

import "github.com/google/uuid"

type TransactionState uint8

const (
	TRANSACTION_PREPARED  TransactionState = 1
	TRANSACTION_COMMITTED TransactionState = 2
	TRANSACTION_ABORTED   TransactionState = 3
)

type Entry struct {
	TxID     uuid.UUID
	State    TransactionState
	SenderID int
	Value    int
}
