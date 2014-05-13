// Package secrets provides a utility method to store and retrieve secret
// strings in an App Engine app, so they don't need to be stored in source.
//
// Secret values should be set in the datastore, and will be cached in memcache
// and local instance memory for quick lookup. Values should not be changed once
// they are set.
//
// Intended use cases are OAuth client secrets, for instance, which are used
// across many requests are should be quick to look up, but shouldn't be stored
// in source control as consts.
//
// Values are not encrypted or obfuscated, and will be easily visible to any
// other app admin.
package secrets

import (
	"appengine"
	"appengine/datastore"
	"appengine/memcache"
)

// Entity name used to store secrets in the datastore.
var EntityName = "Secrets"

// Prefix to use when storing secrets in memcache.
var MemcacheKeyPrefix = ""

var local = map[string]string{}

// Get returns the secret value associated with the key.
//
// If the value cannot be found in any of local instance memory, memcache, or
// the datastore, an empty string will be returned.
func Get(c appengine.Context, key string) string {
	if v, ok := local[key]; ok {
		// Found secret in instance memory, return it.
		return v
	}

	i, err := memcache.Get(c, key)
	if err == nil {
		// Found secret in memcache, return it.
		return string(i.Value)
	}

	if err != nil && err != memcache.ErrCacheMiss {
		c.Errorf("error getting %q from memcache: %v", key, err)
	}

	// Get secret from datastore if missing from memcache.
	k := datastore.NewKey(c, EntityName, key, 0, nil)
	var e struct {
		Value string `datastore:"-"`
	}
	if err := datastore.Get(c, k, &e); err != nil {
		c.Errorf("error getting %q from datastore: %v", key, err)
		return ""
	}

	// Store secret in instance memory for next time.
	local[key] = e.Value

	// Store secret in memcache for next time.
	if err := memcache.Set(c, &memcache.Item{
		Key:   MemcacheKeyPrefix + key,
		Value: []byte(e.Value),
	}); err != nil {
		c.Errorf("error setting %q in memcache: %v", key, err)
	}

	return e.Value
}
