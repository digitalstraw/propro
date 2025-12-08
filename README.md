# propro

[![Release](https://img.shields.io/github/v/release/digitalstraw/propro)](https://github.com/digitalstraw/propro/releases)
[![License](https://img.shields.io/github/license/digitalstraw/propro)](/LICENSE)
[![CI](https://github.com/digitalstraw/propro/actions/workflows/ci.yaml/badge.svg)](https://github.com/digitalstraw/propro/actions/workflows/ci.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/digitalstraw/propro)](https://goreportcard.com/report/github.com/digitalstraw/propro)
[![Coverage](https://coveralls.io/repos/github/digitalstraw/propro/badge.svg?branch=main)](https://coveralls.io/github/digitalstraw/propro?branch=main)

`propro` stands for **Protected Properties**. It prevents direct writes to public/exported properties of structs that represent 
entities. It is usable especially in DDD (Domain-Driven Design) contexts where entities in the domain layer should enforce business rules
and [invariants](https://ddd-practitioners.com/home/glossary/business-invariant/) through methods rather than allowing direct
property manipulation.

Typical solution to this problem before `propro` was to keep entity properties private (unexported) and create separate model/DTO 
structs for ORM, JSON serialization and any other purpose that requires access to exported properties. 



## Why propro?

ORMs like GORM or tools like JSON encoding require access to exported struct fields. However, when a domain entity is updated, 
related business logic may need to run and domain events may need to be raised. Bypassing this logic through direct writes to 
entity properties can compromise domain and data integrity.

To enforce domain state mutation through methods, developers often keep entity properties private and create separate “model” or DTO 
structs for GORM or JSON with public properties. This approach has several drawbacks: it introduces significant boilerplate
(mapping code between entity and model/DTO in both directions + related unit tests), requires maintaining 2 or 3 parallel structures 
(entity, model and eventually also the DTO for API output), and adds extra unit tests for mapping integrity in both directions 
(entity -> model, model -> entity).

`propro` solves this at the **linter level** by preventing direct writes to exported properties of structs which the developer selected
to be protected by this linter. This allows entities to keep their properties exported and still be used directly with GORM, 
JSON and similar tools. Allowing linter-controlled access to entity properties eliminates the need for separate 
model/DTO structs and mapping code between them, reducing boilerplate and maintenance overhead.



## Configuration in golangci-lint 
`.golangci.yml`: 
```yaml
settings:
  propro:
    entity-list-file: path/to/entity_list.go
    structs:
      - User
      - Order
```

- **`entity-list-file`** may contain path to a go file containing **`EntityList`** variable with the list of empty pointers to 
  the protected structs.
  - Such a list is required for database migration purposes by ORM tools. 
  - Example content of such a go file:
    ```go
    var EntityList = []any{
        &users.User{},
        &users.Order{},
    }
    ```
  - The file may be empty or not present, and then this configuration option is ignored.


- **`structs`**: may contain a list of struct names that should be protected. May be empty or not present.  


If both `entity-list-file` and `structs` are specified, the union of the two sets is used. If neither is specified, 
the linter **protects ALL STRUCTS** in the analyzed packages. If you don't want any structs to be protected, just disable the linter.



## Usage with golangci-lint
IMPORTANT: this linter is not part of golangci-lint, the PR was declined but discussion is ongoing yet. 
Feel free to add your comments [in the PR](https://github.com/golangci/golangci-lint/pull/6236).

To use `propro` with `golangci-lint` now, you need to build the linter binary [from the fork](https://github.com/digitalstraw/golangci-lint/tree/add-propro-linter) 
and place it in your `$GOPATH/bin` or `$PATH`. 

```bash
git clone git@github.com:digitalstraw/golangci-lint.git
cd golangci-lint
git checkout add-propro-linter
go build -o golangci-lint ./cmd/golangci-lint
mv golangci-lint $GOPATH/bin/
```

## Go Get
```bash
go get github.com/digitalstraw/propro/v2/
```


## Usage as the Standalone CLI Tool
```bash
git clone git@github.com:digitalstraw/propro.git
go build -o propro cmd/propro/main.go
mv propro $GOPATH/bin/
propro -test=false -entityListFile=./some/path/entity_config.go -structs=Entity1,Entity2 ./...
```

Available CLI parameters:
- `-entityListFile string` - path to a go file containing `EntityList` variable with the list of protected structs.
- `-structs string` - comma-separated list of struct names to be protected.
- `-test bool` - whether to run on test files. This flag is provided by the driver, not the analyzer. Default 
  is `true` and it is recommended to turn it off.




## Code Examples
```go
type Entity struct {
	ProtectedField int
}

func (e *Entity) SetProtectedField(value int) {
	e.ProtectedField = value
}

func SomeFunc1() {
    e := &Entity{}
    
    e.SetProtectedField(10) // OK
    a := e.ProtectedField + 10 // OK
    
    e.ProtectedField = 10 // Error
    e.ProtectedField += 5 // Error
    e.ProtectedField++ // Error
    *(&e.ProtectedField)-- // Error
    b := &e.ProtectedField // Error
}
```



## Limitations
These edge cases are intentionally not covered by this linter:
- indirect modifications through pointers returned by methods,
- adding pointer to the property to a slice or map,
- passing the property value pointer to the channel,
- using reflection to modify the property,
- using `unsafe` package to modify the property directly,
- modifying properties in assembly code.

These unsafe paths exist but are uncommon and intentionally excluded to avoid false positives and excessive complexity.



## Pull Requests
- Feel free to open pull requests for bug fixes and new features; please include tests.
- Use semantic prefixes for commit messages to determine version bumps:
  - `Fix:` for bug fixes -> bumps patch version
  - `New:` for new features -> bumps minor version
  - `Breaking:` for breaking changes -> bumps major version
  - `Chore:` for changes that do not modify src or test files
  - Anything else will be considered as `Fix:`
