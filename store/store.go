package store

import (
	"fmt"

	"github.com/loomnetwork/go-loom/plugin"
	"github.com/loomnetwork/go-loom/util"
	"github.com/loomnetwork/loomchain/log"
)

// KVReader interface for reading data out of a store
type KVReader interface {
	// Get returns nil iff key doesn't exist. Panics on nil key.
	Get(key []byte) []byte

	// Range returns a range of keys
	Range(prefix []byte) plugin.RangeData

	// Has checks if a key exists.
	Has(key []byte) bool
}

type KVWriter interface {
	// Set sets the key. Panics on nil key.
	Set(key, value []byte)

	// Delete deletes the key. Panics on nil key.
	Delete(key []byte)
}

type KVStore interface {
	KVReader
	KVWriter
}

type KVStoreTx interface {
	KVStore
	Commit()
	Rollback()
}

type AtomicKVStore interface {
	KVStore
	BeginTx() KVStoreTx
}

type Snapshot interface {
	KVReader
	Release()
}

type VersionedKVStore interface {
	KVStore
	Hash() []byte
	Version() int64
	SaveVersion() ([]byte, int64, error)
	// Delete old version of the store
	Prune() error
	GetSnapshot() Snapshot
}

type cacheItem struct {
	Value   []byte
	Deleted bool
}

type txAction int

const (
	txSet txAction = iota
	txDelete
)

type tempTx struct {
	Action     txAction
	Key, Value []byte
}

// cacheTx is a simple write-back cache
type cacheTx struct {
	store KVStore
	cache map[string]cacheItem
	// tmpTxs preserves the order of set and delete actions
	tmpTxs []tempTx
}

func newCacheTx(store KVStore) *cacheTx {
	c := &cacheTx{
		store: store,
	}
	c.Rollback()
	return c
}

func (c *cacheTx) addAction(action txAction, key, value []byte) {
	c.tmpTxs = append(c.tmpTxs, tempTx{
		Action: action,
		Key:    key,
		Value:  value,
	})
}

func (c *cacheTx) setCache(key, val []byte, deleted bool) {
	c.cache[string(key)] = cacheItem{
		Value:   val,
		Deleted: deleted,
	}
}

func (c *cacheTx) Delete(key []byte) {
	c.addAction(txDelete, key, nil)
	c.setCache(key, nil, true)
}

func (c *cacheTx) Set(key, val []byte) {
	c.addAction(txSet, key, val)
	c.setCache(key, val, false)
}

func (c *cacheTx) Range(prefix []byte) plugin.RangeData {
	//TODO cache ranges???
	return c.store.Range(prefix)
}

func (c *cacheTx) Has(key []byte) bool {
	if item, ok := c.cache[string(key)]; ok {
		return !item.Deleted
	}

	return c.store.Has(key)
}

func (c *cacheTx) Get(key []byte) []byte {
	if item, ok := c.cache[string(key)]; ok {
		return item.Value
	}

	return c.store.Get(key)
}

func (c *cacheTx) Commit() {
	commits := 0
	deletes := 0
	commitBytes := 0
	for _, tx := range c.tmpTxs {
		if tx.Action == txSet {
			c.store.Set(tx.Key, tx.Value)
			commits = commits + 1
			commitBytes = commitBytes + len(tx.Value)
		} else if tx.Action == txDelete {
			c.store.Delete(tx.Key)
			deletes = deletes + 1
		} else {
			panic("invalid cacheTx action type")
		}
	}
	print := fmt.Sprintf("Commit- %d sets(%d bytes), %d deletes\n", commits, commitBytes, deletes)
	fmt.Printf(print)
	log.Error(print)
}

func (c *cacheTx) Rollback() {
	c.tmpTxs = make([]tempTx, 0)
	c.cache = make(map[string]cacheItem)
}

type atomicWrapStore struct {
	KVStore
}

func (a *atomicWrapStore) BeginTx() KVStoreTx {
	return newCacheTx(a)
}

func WrapAtomic(store KVStore) AtomicKVStore {
	return &atomicWrapStore{
		KVStore: store,
	}
}

type prefixReader struct {
	prefix []byte
	reader KVReader
}

func (r *prefixReader) Range(prefix []byte) plugin.RangeData {
	return r.reader.Range(util.PrefixKey(r.prefix, prefix))
}

func (r *prefixReader) Get(key []byte) []byte {
	log.Error("Get with prefix", "prefix", string(r.prefix), "key", string(key))
	return r.reader.Get(util.PrefixKey(r.prefix, key))
}

func (r *prefixReader) Has(key []byte) bool {
	return r.reader.Has(util.PrefixKey(r.prefix, key))
}

func PrefixKVReader(prefix []byte, reader KVReader) KVReader {
	return &prefixReader{
		prefix: prefix,
		reader: reader,
	}
}

type prefixWriter struct {
	prefix []byte
	writer KVWriter
}

func (w *prefixWriter) Set(key, val []byte) {
	log.Error("Set with prefix", "prefix", string(w.prefix), "key", string(key))
	w.writer.Set(util.PrefixKey(w.prefix, key), val)
}

func (w *prefixWriter) Delete(key []byte) {
	w.writer.Delete(util.PrefixKey(w.prefix, key))
}

func PrefixKVWriter(prefix []byte, writer KVWriter) KVWriter {
	return &prefixWriter{
		prefix: prefix,
		writer: writer,
	}
}

type prefixStore struct {
	prefixReader
	prefixWriter
}

func PrefixKVStore(prefix []byte, store KVStore) KVStore {
	return &prefixStore{
		prefixReader{
			prefix: prefix,
			reader: store,
		},
		prefixWriter{
			prefix: prefix,
			writer: store,
		},
	}
}
