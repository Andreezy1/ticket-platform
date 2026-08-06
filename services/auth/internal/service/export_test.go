package service

func SetDummyHash(h []byte) {
	dummyHash = h
}

func SetBcryptCost(c int) {
	bcryptCost = c
}
