# response

Response is simple helper for rest api responses. The naming of api functions is: for setter `Value` and for getter
`GetValue` (Only exception is `String` so we satisfy `Stringer` interface).
Let's see some examples:

```go
response.New(http.StatusOK).Write(w, r)

// or with default OK status

response.New().Write(w, r)
    
```

This writes following response to ResponseWriter (w).

```json
{
  "status": 200,
  "message": "OK"
}
```

Response adds ability to add various data to response object. You can use method `Data` to set data
or there are some shorthand for usual data keys.

```go

response.New().Data("result", result).Write(w, r)

// or with a shorthand

response.New().Result(result).Write(w, r)
```

There is also shorthand to provide slice result that automatiaclly adds `result_size`.

```go
response.New().SliceResult(result).Write(w, r)
```
### Shortcuts

response serves following shortcuts to create responses 
* Data
* Error
* HTML
* Result
* SliceResult
* Write

Example:

```go
result := map[string]string{}
response.Result(result)
````

Is the same as doing 
```go
result := map[string]string{}
response.New(http.StatusOK).Result(result)
````

or even
```go
result := map[string]string{}
response.New().Result(result)
````

### Contributions

You are welcome to send PR.

