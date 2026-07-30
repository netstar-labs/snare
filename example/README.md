# snare examples

| Example | What it shows | Run |
|---|---|---|
| [near](near/main.go) | building a `Set` over brand targets and querying look-alikes — a substitution, a transposition, an insertion, an exact brand (no hit), and a combosquat (no hit) | `go run ./example/near` |

Build standalone with `GOWORK=off` if the surrounding workspace doesn't list this
module.

For the CLI over a targets file and queries from arguments or stdin, see
[../docs/userguide.md](../docs/userguide.md):

```sh
go run ./app/snare near -t targets.txt paypa1 papyal paypal   # nearest per query
printf 'paypa1\npapyal\n' | go run ./app/snare near -t targets.txt   # queries on stdin
```
