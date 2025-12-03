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
- **`entityListFile`** may contain path to a go file containgin **`EntityList`** variable with the list of empty pointers to the protected structs.
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
        structs:
          - "User"
          - "Order"
    ```

- **`skipTests`**: if set to true, the linter will skip test files.

If both `entityListFile` and `structs` are specified, the union of the two sets is used.
If neither is specified, the linter **protects ALL STRUCTS**. If you don't want any structs to be protected, just disable the linter.


## Usage
```bash
go get github.com/digitalstraw/propro/cmd/propro

propro .
```
Available parameters:
- `-entityListFile string` - path to a go file containing `EntityList` variable with the list of protected structs.
- `-structs string` - comma-separated list of struct names to be protected.
- `-skipTests bool` - if set, test files will be skipped. `false` by default.


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
    a := e.ProtectedField + 10; // OK
	
    e.ProtectedField = 10 // Error
    e.ProtectedField += 5 // Error
    e.ProtectedField++ // Error
	*(&e.ProtectedField)-- // Error
    b := &e.ProtectedField // Error
}
```
