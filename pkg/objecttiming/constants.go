package objecttiming

/* Flags for each action */
const (
	ParseCreate	= 1 << iota
	ParseScale
	ParseDelete
	ParseResize
)

/* Unique values for each action */
const (
	opCreate = iota
	opScale
	opDelete
	opResize
)

/* Constants for the string names of each operation */
const (
	strCreate = "create"
	strScale = "scale"
	strDelete = "delete"
	strResize = "resize"
)
