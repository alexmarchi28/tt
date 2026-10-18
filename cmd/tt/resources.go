package main

// resourceReader reads the themes, word lists and quotes of tt, see
// resource.Layers.
type resourceReader interface {
	// Read returns the resource of the given type (themes, words or quotes)
	// and name.
	Read(typ, name string) ([]byte, error)
}

// resourceLister also lists the resources of a type.
type resourceLister interface {
	resourceReader
	List(typ string) []string
}
