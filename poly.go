package poly

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"gopkg.in/yaml.v3"
)

var (
	ErrMissingDiscriminator = errors.New("missing discriminator")
	ErrMissingTypeFor       = errors.New("invalid or missing type for discriminator")
	ErrInvalidJSON          = errors.New("invalid discriminator json")

	DataNone = []byte("[]")

	byType                     map[reflect.Type]string
	byTypeSpecialized          map[reflect.Type]map[reflect.Type]string
	byDiscriminator            map[string]reflect.Type
	byDiscriminatorSpecialized map[reflect.Type]map[string]reflect.Type
)

func init() {
	Reset()
}

// Clears out all registered discriminators.
func Reset() {
	byType = make(map[reflect.Type]string)
	byTypeSpecialized = make(map[reflect.Type]map[reflect.Type]string)
	byDiscriminator = make(map[string]reflect.Type)
	byDiscriminatorSpecialized = make(map[reflect.Type]map[string]reflect.Type)
}

// Registers a discriminator for the given type. This is the fallback/general
// discriminator. A specialized one can be set with RegisterSpecialized.
func Register[S any](discriminator string) {
	typ := reflect.TypeFor[S]()
	byType[typ] = discriminator
	byDiscriminator[discriminator] = typ
}

// Registers a discriminator for type S which implements interface P.
// Type S may have other discriminators, but when the polymorphic type instance
// uses interface P it will use this discriminator.
func RegisterSpecialized[P any, S any](discriminator string) {
	typT := reflect.TypeFor[S]()
	typS := reflect.TypeFor[P]()
	if _, specialExists := byTypeSpecialized[typS]; !specialExists {
		byTypeSpecialized[typS] = make(map[reflect.Type]string)
	}
	if _, specialExists := byDiscriminatorSpecialized[typS]; !specialExists {
		byDiscriminatorSpecialized[typS] = make(map[string]reflect.Type)
	}
	byTypeSpecialized[typS][typT] = discriminator
	byDiscriminatorSpecialized[typS][discriminator] = typT
}

// Creates a polymorphic instance for interface P.
func C[P any](value P) T[P] {
	return T[P]{Value: value}
}

// Creates a pointer to a polymorphic instance for interface P.
func P[P any](value P) *T[P] {
	return &T[P]{Value: value}
}

// A polymorphic instance for interface P.
type T[P any] struct {
	Value P
}

var _ json.Marshaler = T[any]{}
var _ json.Unmarshaler = &T[any]{}
var _ yaml.Marshaler = T[any]{}
var _ yaml.Unmarshaler = &T[any]{}

// Returns the discriminator for the value in this polymorphic type.
// If no Value is defined then "" will be returned.
func (d T[P]) Discriminator() string {
	valueR := reflect.ValueOf(d.Value)
	if !valueR.IsValid() || valueR.IsZero() {
		return ""
	}

	valueT := reflect.TypeOf(d.Value)
	specialT := reflect.TypeFor[P]()
	discriminator := ""
	if special, ok := byTypeSpecialized[specialT]; ok {
		discriminator = special[valueT]
	}
	if discriminator == "" {
		discriminator = byType[valueT]
	}

	return discriminator
}

// Returns a new *P value for the discriminator. If there is no valid value
// for the discriminator OR it does not implement P then nil will be returned.
func (d T[P]) Discriminated(discriminator string) (P, bool) {
	specialP := reflect.TypeFor[P]()
	var valueP reflect.Type
	if special, exists := byDiscriminatorSpecialized[specialP]; exists {
		valueP = special[discriminator]
	}
	if valueP == nil {
		valueP = byDiscriminator[discriminator]
	}
	var emptyP P
	if valueP == nil {
		return emptyP, false
	}

	if valueP.Kind() == reflect.Pointer {
		valueP = valueP.Elem()
	}
	valueI := reflect.New(valueP).Interface()
	if valueP, ok := valueI.(P); ok {
		return valueP, true
	}

	return emptyP, false
}

// Returns whether this is a zero-value polymorphic type.
// It's considered zero-value if the value matches the empty value for type P
func (d T[P]) IsZero() bool {
	rv := reflect.ValueOf(d.Value)
	return !rv.IsValid() || rv.IsZero()
}

//===================================================================
// JSON: stored in the format of `['discriminator', value]`
//===================================================================

func (d T[P]) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return DataNone, nil
	}

	discriminator := d.Discriminator()
	if discriminator == "" {
		return nil, fmt.Errorf("%w: for %v of %v", ErrMissingDiscriminator, reflect.TypeOf(d.Value), reflect.TypeFor[P]())
	}

	return json.Marshal([]any{
		discriminator,
		d.Value,
	})
}

func (d *T[P]) UnmarshalJSON(b []byte) error {
	if bytes.Equal(b, DataNone) {
		return nil
	}

	dec := json.NewDecoder(bytes.NewReader(b))

	t, err := dec.Token()
	if err != nil {
		return err
	}
	if t != json.Delim('[') {
		return fmt.Errorf("%w: %v", ErrInvalidJSON, t)
	}

	t, err = dec.Token()
	if err != nil {
		return err
	}
	discriminator := ""
	if s, ok := t.(string); ok {
		discriminator = s
	} else {
		return fmt.Errorf("%w: expected string but got %v", ErrInvalidJSON, t)
	}

	discriminated, ok := d.Discriminated(discriminator)
	if !ok {
		return fmt.Errorf("%w: %s of %v", ErrMissingTypeFor, discriminator, reflect.TypeFor[P]())
	}

	valueB := b[dec.InputOffset()+1 : len(b)-1]

	err = json.Unmarshal(valueB, &discriminated)
	if err != nil {
		return err
	}

	d.Value = discriminated

	return nil
}

//===================================================================
// JSON: stored in the format of:
// property:
//   - discriminator
//   - value
//===================================================================

func (d T[P]) MarshalYAML() (any, error) {
	// Contains no polymorphic value
	if d.IsZero() {
		return nil, nil
	}

	discriminator := d.Discriminator()
	if discriminator == "" {
		return nil, fmt.Errorf("%w: for %v of %v", ErrMissingDiscriminator, reflect.TypeOf(d.Value), reflect.TypeFor[P]())
	}

	return []any{
		discriminator,
		d.Value,
	}, nil
}

func (d *T[P]) UnmarshalYAML(value *yaml.Node) error {
	// Contains no polymorphic value
	if value.IsZero() {
		return nil
	}

	pair := [2]yaml.Node{}
	err := value.Decode(&pair)
	if err != nil {
		return err
	}

	discriminator := pair[0].Value
	if discriminator == "" {
		return fmt.Errorf("%w: for %v of %v", ErrMissingDiscriminator, reflect.TypeOf(d.Value), reflect.TypeFor[P]())
	}

	discriminated, ok := d.Discriminated(discriminator)
	if !ok {
		return fmt.Errorf("%w: %s of %v", ErrMissingTypeFor, discriminator, reflect.TypeFor[P]())
	}
	err = pair[1].Decode(discriminated)
	if err != nil {
		return err
	}
	d.Value = discriminated

	return nil
}
