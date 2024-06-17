package rules

import (
	"fmt"
	"time"

	"github.com/appbaseio-confidential/reactivesearch/plugins/cache"
	"github.com/dgraph-io/ristretto"
	"rogchap.com/v8go"
)

type KV struct {
	cache *ristretto.Cache
}

// GetKVWriteFunctionCallback will return the callback function
// to be injected into v8go context.
//
// This function will expose a way to store the passed value
// against the passed key in badger.
func (kv *KV) GetKVWriteFunctionCallback() v8go.FunctionCallback {
	return func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		args := info.Args()
		ctx := info.Context()

		if len(args) != 2 {
			// Use throwException to throw the error.
			errorForJS, _ := v8go.NewValue(ctx.Isolate(), "`key` and `value` are required params for this function, cannot continue!")
			ctx.Isolate().ThrowException(errorForJS)
			return nil
		}

		keyAsString := args[0].String()
		valueAsString := args[1].String()

		// Store the string in badger and return nothing
		valueStoreErr := kv.StoreValueInCache(keyAsString, valueAsString)
		if valueStoreErr == nil {
			// Return nil value since the value was added successfully
			return v8go.Null(ctx.Isolate())
		}

		// Throw exception since there was some error.
		errorForJS, _ := v8go.NewValue(ctx.Isolate(), valueStoreErr.Error())
		ctx.Isolate().ThrowException(errorForJS)
		return nil
	}
}

// GetKVReadFunctionCallback will return the callback function
// to be injected into v8go context.
//
// This function will expose a way to fetch a value for a passed
// key.
func (kv *KV) GetKVReadFunctionCallback() v8go.FunctionCallback {
	return func(info *v8go.FunctionCallbackInfo) *v8go.Value {
		args := info.Args()
		ctx := info.Context()

		if len(args) != 1 {
			// Use throwException to throw the error.
			errorForJS, _ := v8go.NewValue(ctx.Isolate(), "`key` is a required param for this function, cannot continue!")
			ctx.Isolate().ThrowException(errorForJS)
			return nil
		}

		keyAsString := args[0].String()

		valueRead, isFound, valueReadErr := kv.GetValueFromCache(keyAsString)
		if valueReadErr != nil {
			// Use throwException to throw the error.
			errorForJS, _ := v8go.NewValue(ctx.Isolate(), fmt.Sprintf("error while reading value for the passed key: %s", valueReadErr.Error()))
			ctx.Isolate().ThrowException(errorForJS)
			return nil
		}

		// If it is not found, return nil
		if !isFound {
			return v8go.Null(ctx.Isolate())
		}

		valueAsStr, _ := v8go.NewValue(ctx.Isolate(), valueRead)
		return valueAsStr
	}
}

// StoreValueInCache will store the passed value against the
// passed key in ristretto.
//
// The TTL of the key will be set according to the cache preferences
// of the cluster.
func (kv *KV) StoreValueInCache(key, value string) error {
	cachePreferences := cache.GetCachePreferences()
	if cachePreferences.EnableCache == nil || !*cachePreferences.EnableCache || kv.cache == nil {
		// Cache is disabled or something else has stopped caching
		// from being initiated, we cannot continue.
		return fmt.Errorf("Caching is disabled. Enable it by heading over to ReactiveSearch Dashboard.")
	}

	ttlSet := time.Duration(*cachePreferences.MaxDuration) * time.Second
	isAdded := kv.cache.SetWithTTL(key, value, int64(len([]byte(value))), ttlSet)

	// Wait for the value to pass through buffers
	kv.cache.Wait()

	if !isAdded {
		return fmt.Errorf("Error while setting key `%s` in cache with value `%s`", key, value)
	}

	return nil
}

// GetValueFromCache will read the value from the passed key
//
// If the key is not found an empty string will be returned.
func (kv *KV) GetValueFromCache(key string) (interface{}, bool, error) {
	cachePreferences := cache.GetCachePreferences()
	if cachePreferences.EnableCache == nil || !*cachePreferences.EnableCache || kv.cache == nil {
		// Cache is disabled or something else has stopped caching
		// from being initiated, we cannot continue.
		return "", false, fmt.Errorf("Caching is disabled. Enable it by heading over to ReactiveSearch Dashboard.")
	}

	value, isFound := kv.cache.Get(key)
	return value, isFound, nil
}

// InjectKV will inject the KV related functions into the passed
// context.
func InjectKV(ctx *v8go.Context) error {
	if ctx == nil {
		return fmt.Errorf("ctx is nil, cannot inject")
	}

	iso := ctx.Isolate()

	kvObj := v8go.NewObjectTemplate(iso)

	cacheObj := cache.GetSearchCache()

	kv := &KV{
		cache: cacheObj,
	}

	readFn := v8go.NewFunctionTemplate(iso, kv.GetKVReadFunctionCallback())
	writeFn := v8go.NewFunctionTemplate(iso, kv.GetKVWriteFunctionCallback())

	// Inject the read/write functions as properties of the object
	readInjectErr := kvObj.Set("get", readFn, v8go.ReadOnly)
	if readInjectErr != nil {
		return fmt.Errorf("error injecting read function: %s", readInjectErr.Error())
	}

	writeInjectErr := kvObj.Set("put", writeFn, v8go.ReadOnly)
	if writeInjectErr != nil {
		return fmt.Errorf("error injecting put function: %s", writeInjectErr.Error())
	}

	kvObjToInject, err := kvObj.NewInstance(ctx)
	if err != nil {
		return fmt.Errorf("error creating object for kv: %s", err.Error())
	}

	global := ctx.Global()

	kvInjectErr := global.Set("KV", kvObjToInject)
	if kvInjectErr != nil {
		return fmt.Errorf("error while inject KV object into context: %s", kvInjectErr.Error())
	}

	return nil
}
