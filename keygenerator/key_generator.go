package keygenerator

type KeyGenerator interface {
	Generate(length int) string
}
