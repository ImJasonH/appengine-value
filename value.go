// +build appengine

// Package value provides a utility method to retrieve string values in an
// App Engine app, so they can be stored in the datastore instead of in source.
//
// Values should be set in the datastore using the App Engine Admin UI, and will
// be cached in memcache and local instance memory for quick lookup. Values
// should not be changed once they are set.
//
// Intended use cases are OAuth client secrets or API keys, for instance, which
// are used across many requests are should be quick to look up, but shouldn't
// be stored in source control as consts for security reasons.
//
// Values are not encrypted or obfuscated, and will be easily visible to any
// other app admin.
package value

import (
	"appengine"
	"appengine/datastore"
	"appengine/memcache"
)

// Entity name used to store values in the datastore.
var EntityName = "Values"

// Prefix to use when storing values in memcache.
var MemcacheKeyPrefix = ""

var local = map[string]string{}

// Get returns the value associated with the key.
//
// If the value cannot be found in any of local instance memory, memcache, or
// the datastore, an empty string will be returned.
func Get(c appengine.Context, key string) string {
	if v, ok := local[key]; ok {
		// Found value in instance memory, return it.
		return v
	}

	i, err := memcache.Get(c, key)
	if err == nil {
		// Found value in memcache, return it.
		return string(i.Value)
	}

	if err != nil && err != memcache.ErrCacheMiss {
		c.Errorf("error getting %q from memcache: %v", key, err)
	}

	// Get value from datastore if missing from memcache.
	k := datastore.NewKey(c, EntityName, key, 0, nil)
	var e struct {
		Value string `datastore:"-"`
	}
	if err := datastore.Get(c, k, &e); err != nil {
		c.Errorf("error getting %q from datastore: %v", key, err)
		return ""
	}

	// Store value in instance memory for next time.
	local[key] = e.Value

	// Store value in memcache for next time.
	if err := memcache.Set(c, &memcache.Item{
		Key:   MemcacheKeyPrefix + key,
		Value: []byte(e.Value),
	}); err != nil {
		c.Errorf("error setting %q in memcache: %v", key, err)
	}

	return e.Value
}
