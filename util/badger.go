package util

import "github.com/outcaste-io/badger/v3"

func SetBadgerOptions(filePath string) badger.Options {
	opts := badger.DefaultOptions(filePath)
	opts.NumMemtables = 2

	// The NumLevelZeroTables and NumLevelZeroTableStall will not have any
	// effect on the memory if `KeepL0InMemory` is set to false.
	opts.NumLevelZeroTables = 1
	opts.NumLevelZeroTablesStall = 2

	// SyncWrites=false has significant effect on write performance. When sync
	// writes is set to true, badger ensures the data is flushed to the disk after a
	// write call. For normal usage, such a high level of consistency is not required.
	opts.SyncWrites = false

	return opts
}
