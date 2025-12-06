# propro

[![Release](https://img.shields.io/github/v/release/digitalstraw/propro)](https://github.com/digitalstraw/propro/releases)
[![License](https://img.shields.io/github/license/digitalstraw/propro)](/LICENSE)
[![CI](https://github.com/digitalstraw/propro/actions/workflows/ci.yaml/badge.svg)](https://github.com/digitalstraw/propro/actions/workflows/ci.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/digitalstraw/propro)](https://goreportcard.com/report/github.com/digitalstraw/propro)
[![Coverage](https://coveralls.io/repos/github/digitalstraw/propro/badge.svg?branch=main)](https://coveralls.io/github/digitalstraw/propro?branch=main)

`propro` stands for **Protected Properties**. It prevents direct writes to public/exported properties of structs that represent domain entities.

## Why propro?

ORMs like GORM or tools like JSON encoding require access to exported struct fields.  
However, when a domain entity is updated, validation logic may need to run and domain events may need to be raised.  
Bypassing this logic through direct writes to entity properties can compromise domain integrity.

To enforce domain state mutation through methods, developers often keep entity properties private and create separate “model” 
structs for GORM or JSON with public properties. This approach has several drawbacks: it introduces significant boilerplate, 
requires maintaining two parallel structures, and adds extra unit tests for mapping logic in both directions.

`propro` solves this at the **linter level** by preventing direct writes to exported properties of structs marked as protected by the user.  
This allows entities to keep their properties exported and still be used directly with GORM and JSON.


## Configuration
- **`entity-list-file`** may contain path to a go file containing **`EntityList`** variable with the list of empty pointers to the protected structs.
  - Such a list is required for database migration purposes by ORM tools like GORM. 
  - Example content of such a go file:
    ```go
    var EntityList = []any{
        &users.User{},
        &users.Order{},
    }
    ```
  - The file may be empty or not present, and then this configuration option is ignored.


- **`structs`**: may contain a list of struct names that should be protected. May be empty or not present.  
  - Example in `.golangci.yml`: 
    ```yaml
    settings:
      propro:
        entity-list-file: "path/to/entity_list.go"
        structs:
          - "User"
          - "Order"
    ```

If both `entity-list-file` and `structs` are specified, the union of the two sets is used.
If neither is specified, the linter **protects ALL STRUCTS**. If you don't want any structs to be protected, just disable the linter.


## Usage
```bash
go get github.com/digitalstraw/propro/cmd/propro
go build -o propro cmd/propro/main.go

propro .
```
Available CLI parameters:
- `-entityListFile string` - path to a go file containing `EntityList` variable with the list of protected structs.
- `-structs string` - comma-separated list of struct names to be protected.


## Code Examples
```go
type Entity struct {
	ProtectedField int
}

func (e *Entity) SetProtectedField(value string) {
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

## Pull Requests
- Feel free to open pull requests for bug fixes and new features; please include tests.
- Use **semantic versioning** for commit messages.
- Use the following prefixes for commit messages:
  - `Fix:` for bug fixes -> bumps patch version
  - `New:` for new features -> bumps minor version
  - `Breaking:` for breaking changes -> bumps major version
  - `Chore:` for changes that do not modify src or test files
  - Anything else will be considered as `Fix:`
  - Note: prefixes are **case-insensitive** and does not have to be followed by colon, but it is recommended.

## Note
Some use-cases are intentionally not covered by this linter. See tests.
