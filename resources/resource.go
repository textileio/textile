package resources

type Resource interface {
	StoreID() string
	Create(docs ...interface{}) error
	Get(id string, doc interface{}) error
	List(dummy interface{}) (interface{}, error)
	Update(doc interface{}) error
	Delete(id string) error
}
