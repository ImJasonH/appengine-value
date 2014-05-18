// +build appengine

package value

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"appengine"
	"appengine/datastore"
	"appengine/memcache"
	"appengine/user"
)

func init() {
	http.HandleFunc("/_ah/value/admin", adminHandler)
	http.HandleFunc("/_ah/value/update", updateHandler)
}

var adminTmpl = template.Must(template.New("admin").Parse(`<html><body>
<h1>Admin</h1>
<table>
{{range $key, $val := .}}
  <form action="/_ah/value/update" method="POST">
    <tr>
    <input type="hidden" name="delete_key" value="{{$key}}"></input>
    <td>{{$key}}</td>
    <td>{{$val}}</td>
    <td><input type="submit" value="Delete"></input></td>
    </tr>
  </form>
{{else}}
  <tr><td colspan="3"><center><b>no values currently configured</b></center><td></tr>
{{end}}
<form action="/_ah/value/update" method="POST">
  <tr>
  <td><input type="text" name="key"></input></td>
  <td><input type="text" name="val"></input></td>
  <td><input type="submit" value="Add"></input></td>
  </tr>
</form>
</table>
</body></html>`))

func adminHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	if !user.IsAdmin(c) {
		if user.Current(c) == nil {
			loginURL, _ := user.LoginURL(c, "/_ah/value/admin")
			fmt.Fprintf(w, "<a href='%s'>Log in</a>", loginURL)
		} else {
			http.Error(w, "Forbidden", http.StatusForbidden)
		}
		return
	}
	if r.Method != "GET" {
		return
	}
	v := map[string]string{}
	q := datastore.NewQuery(Kind)
	for t := q.Run(c); ; {
		var e e
		k, err := t.Next(&e)
		if err == datastore.Done {
			break
		} else if err != nil {
			c.Errorf("%v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		v[k.StringID()] = e.Value
	}
	if err := adminTmpl.Execute(w, v); err != nil {
		c.Warningf("error executing template: %v", err)
	}
}

func updateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Redirect(w, r, "/_ah/value/admin", http.StatusSeeOther)
		return
	}
	c := appengine.NewContext(r)
	if !user.IsAdmin(c) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	deleteKey := r.FormValue("delete_key")
	if deleteKey != "" {
		if err := memcache.Delete(c, deleteKey); err != nil && err != memcache.ErrCacheMiss {
			c.Errorf("error deleting %q from memcache: %v", deleteKey, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		k := datastore.NewKey(c, Kind, deleteKey, 0, nil)
		if err := datastore.Delete(c, k); err != nil {
			c.Errorf("error deleting %q from datastore: %v", deleteKey, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		key := r.FormValue("key")
		val := r.FormValue("val")
		if err := set(c, key, val); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	// TODO: remove this hack.
	time.Sleep(time.Millisecond * 500)
	http.Redirect(w, r, "/_ah/value/admin", http.StatusSeeOther)
}

func set(c appengine.Context, key string, val string) error {
	if _, err := memcache.Get(c, key); err != memcache.ErrCacheMiss {
		return errors.New("key found in memcache")
	}
	return datastore.RunInTransaction(c, func(tc appengine.Context) error {
		// Fail if the value is already stored.
		k := datastore.NewKey(c, Kind, key, 0, nil)
		if err := datastore.Get(tc, k, nil); err == nil {
			return errors.New("key found in datastore")
		} else if err == datastore.ErrNoSuchEntity {
			// Okay, good. Keep going.
		} else if err != nil {
			return err
		}

		// Put the value in the datastore.
		_, err := datastore.Put(tc, k, &e{Value: val})
		return err
	}, nil)
}
