package protectall

type UnProtectedEntity struct {
	UnProtectedField string
}

type Entity struct {
	SubEntityViaProperty *SubEntity
	ProtectedField       string
	IntField             int
	IntPtrField          *int
}

func (e *Entity) SubEntity() *SubEntity {
	return &SubEntity{}
}

func (e *Entity) SubEntityViaInterface() SubEntityInterface {
	return &SubEntity{}
}

type SubEntityInterface interface {
	SetProtectedField(value string)
}

type SubEntity struct {
	ProtectedField string
}

func (s *SubEntity) SetProtectedField(value string) {
	s.ProtectedField = value
}

type Entity2 struct {
	SubEntityViaProperty *SubEntity2
	ProtectedField       string
	IntField             int
}

type SubEntity2 struct {
	Next *SubSubEntity2
}

type SubSubEntity2 struct {
	ProtectedField string
}

type Entity3 struct {
	ProtectedField string
}

type SubEntity3 struct {
	*Entity3
}

type SubSubEntity3 struct {
	*SubEntity3
}

type Entity4 struct {
	ProtectedField string
}

type SubEntity4 struct {
	e *Entity4
}

type SubSubEntity4 struct {
	e *SubEntity4
}

type Repository interface {
	Read() *Entity
}

type RepositoryImpl struct{}

func (r *RepositoryImpl) Read() *Entity {
	return &Entity{}
}

var repo Repository = &RepositoryImpl{}

func (e *Entity) SetProtectedField(value string) {
	e.ProtectedField = value
}

type SubEntityWithPtrComposition struct {
	*Entity

	ProtectedField string
}

type SubEntityWithComposition struct {
	Entity

	ProtectedField string
}

func SomeFunc1() {
	e := &Entity{}
	e.SetProtectedField("value")
}

func SomeFunc2() {
	e := &Entity{}
	e.ProtectedField = "value" // want "assignment to exported field Entity.ProtectedField is forbidden outside its methods"
}

func SomeFunc3() {
	e := repo.Read()
	e.ProtectedField = "value" // want "assignment to exported field Entity.ProtectedField is forbidden outside its methods"
}

func SomeFunc4() {
	e := repo.Read()
	e.SetProtectedField("value")
}

func SomeFunc5() {
	e := &Entity{}
	sub := e.SubEntity()
	sub.SetProtectedField("value")
}
func SomeFunc6() {
	e := &Entity{}
	sub := e.SubEntity()
	sub.ProtectedField = "value" // want "assignment to exported field SubEntity.ProtectedField is forbidden outside its methods"
}

func SomeFunc7() {
	e := &Entity{}
	sub := e.SubEntityViaInterface()
	sub.SetProtectedField("value")
}

func SomeFunc8() {
	e := &Entity{
		SubEntityViaProperty: &SubEntity{},
	}
	e.SubEntityViaProperty.ProtectedField = "value" // want "assignment to exported field SubEntity.ProtectedField is forbidden outside its methods"
}

func SomeFunc9() {
	e := &Entity{
		SubEntityViaProperty: &SubEntity{},
	}
	e.SubEntityViaProperty.SetProtectedField("value")
	if e.SubEntityViaProperty.ProtectedField != "value" {
	}
}

func SomeFunc10() {
	// In this version, this will be protected by default in TestWithNoParameters_allStructsAreProtected.
	e := &UnProtectedEntity{}
	e.UnProtectedField = "value" // want "assignment to exported field UnProtectedEntity.UnProtectedField is forbidden outside its methods"
}

func SomeFunc11() {
	e := &SubEntityWithPtrComposition{}
	e.Entity = &Entity{
		ProtectedField: "initial",
	}
	e.ProtectedField = "value" // want "assignment to exported field SubEntityWithPtrComposition.ProtectedField is forbidden outside its methods"
}

func SomeFunc12() {
	e := &SubEntityWithComposition{}
	e.Entity = Entity{
		ProtectedField: "initial",
	}
	e.ProtectedField = "value" // want "assignment to exported field SubEntityWithComposition.ProtectedField is forbidden outside its methods"
}

func SomeFunc13() {
	e := &SubEntityWithPtrComposition{}
	e.Entity = &Entity{}
	e.Entity.ProtectedField = "initial" // want "assignment to exported field Entity.ProtectedField is forbidden outside its methods"
	e.ProtectedField = "value"          // want "assignment to exported field SubEntityWithPtrComposition.ProtectedField is forbidden outside its methods"
}

func SomeFunc14() {
	e := &SubEntityWithComposition{}
	e.Entity = Entity{}
	e.Entity.ProtectedField = "initial" // want "assignment to exported field Entity.ProtectedField is forbidden outside its methods"
	e.ProtectedField = "value"          // want "assignment to exported field SubEntityWithComposition.ProtectedField is forbidden outside its methods"
}

func SomeFunc15() {
	e := &Entity{
		IntField: 5,
	}
	e.IntField++     // want "assignment to exported field Entity.IntField is forbidden outside its methods"
	e.IntField--     // want "assignment to exported field Entity.IntField is forbidden outside its methods"
	*(&e.IntField)++ // want "assignment to exported field Entity.IntField is forbidden outside its methods"
	*(&e.IntField)-- // want "assignment to exported field Entity.IntField is forbidden outside its methods"
	e.IntField = 10  // want "assignment to exported field Entity.IntField is forbidden outside its methods"
	e.IntField += 10 // want "assignment to exported field Entity.IntField is forbidden outside its methods"
	e.IntField -= 10 // want "assignment to exported field Entity.IntField is forbidden outside its methods"
	e.IntField *= 10 // want "assignment to exported field Entity.IntField is forbidden outside its methods"
	e.IntField /= 10 // want "assignment to exported field Entity.IntField is forbidden outside its methods"
	x := &e.IntField // want "assignment to exported field Entity.IntField is forbidden outside its methods"
	*x = 20
	e.IntPtrField = new(int) // want "assignment to exported field Entity.IntPtrField is forbidden outside its methods"

	y := e.IntField + 10
	_ = y
}

func SomeFunc16() {
	e := &Entity2{
		SubEntityViaProperty: &SubEntity2{
			Next: &SubSubEntity2{},
		},
	}
	e.SubEntityViaProperty.Next.ProtectedField = "value" // want "assignment to exported field SubSubEntity2.ProtectedField is forbidden outside its methods"
}

func SomeFunc17() {
	e := &SubSubEntity3{
		SubEntity3: &SubEntity3{
			Entity3: &Entity3{},
		},
	}
	e.ProtectedField = "value" // want "assignment to exported field SubSubEntity3.ProtectedField is forbidden outside its methods"
}

func SomeFunc18() {
	e := &SubSubEntity4{
		e: &SubEntity4{
			e: &Entity4{},
		},
	}
	e.e.e.ProtectedField = "value"     // want "assignment to exported field Entity4.ProtectedField is forbidden outside its methods"
	*(&e.e.e.ProtectedField) = "value" // want "assignment to exported field Entity4.ProtectedField is forbidden outside its methods"
}
