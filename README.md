`appengine-value` is a simple utility library to handle simple secret configuration values in an App Engine app.

Problem
-----

Your app requires secret values (OAuth secrets, API keys, passwords) and you rightly don't want to specify these values as consts in your source code.

Solution
-----

```
import value "github.com/ImJasonH/appengine-value"
// ...
clientSecret := value.Get("client_secret")
```

That's it! The secret will be retrieved from `datastore`, `memcache`, or a `map` in local instance memory, whichever's faster.

To set or unset values, use the admin UI which is served at `/_ah/value/admin` -- be sure to map this in your `app.yaml`:

```
handlers:
- url: /_ah/value/.*
  script: _go_app
  login: admin
```

(You can always remove the mapping if you're done setting values)
