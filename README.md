# poly
A Go module for encoding polymorphic values. ie marshalling interfaces to and from concrete types in JSON and YAML.

> go get github.com/clickermonkey/poly

You register your concrete types with a discriminator (string) that is globally unique or unique to a specific interface.

#### The encoded JSON looks like this:
```js
// A specified value
["discrminator", encodedValue]
// No specified value
[]
```

#### The encoded YAML looks like this
```yaml
- discriminator
- encodedValue
```
And no specified value in YAML is just `null`


### Global Example
```go
type Job interface { Do() error }

type Email struct { Message string `json:"message"` }
func (e Email) Do() error { return e.Message }

type Save struct {}
func (e Save) Do() error { return "saved" }

type JobData struct {
    Job poly.T[Job] `json:"job,omitzero"`
}

func init() {
    poly.Register[Email]("email")
    poly.Register[Save]("save")

    j := JobData{
        Job: poly.C[Job](Email{
            Message: "Hello World!"
        })
    }

    data, _ := json.Marshal(j)
    // {"job":["email",{"message":"Hello World!"}]}

    j2 := JobData{}
    data2, _ := json.Marshal(j)
    // {"job":[]}
}

```

### Specific Example

To avoid overlapping discriminators, you can register specialized ones - so only types that match the specialization are used.

```go
func init() {
    // Replace Register in simple example with these, and these discriminators will not affect non-Job specializations.
    poly.RegisterSpecialized[Job, Email]("email")
    poly.RegisterSpecialized[Job, Save]("save")

    // ...
}
```
