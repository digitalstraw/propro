# propro

[![Release](https://img.shields.io/github/v/release/digitalstraw/propro)](https://github.com/digitalstraw/propro/releases)
[![License](https://img.shields.io/github/license/digitalstraw/propro)](/LICENSE)
[![CI](https://github.com/digitalstraw/propro/actions/workflows/ci.yaml/badge.svg)](https://github.com/digitalstraw/propro/actions/workflows/ci.yaml)

`propro` is an abbreviation of Protected Properties. It prevents direct writes to public/exported struct properties.

## Why propro?

ORMs like GORM, or JSON needs to access public/exported fields of structs it works with. Therefore, developers use mapping entities with
private properties to _model_ structs which contain public properties. However, this approach has drawbacks: it introduces a lot of 
boilerplate code because it requires writing and maintaining mapping logic for both directions, and adds extra unit tests for those mappings.

`propro` solves this problem at the linter level by blocking direct writes to public fields of protected structs.  
This allows entities to keep exported fields and be used directly with GORM and JSON—without all the mapping boilerplate.

## Configuration
- **`entity-list-file`** may contain path to a go file containgin **`EntityList`** variable with the list of empty pointers to the protected structs.
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
