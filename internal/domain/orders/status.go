package orders

// transitions é a única fonte da regra de fluxo do pedido.
// Cancelar só é possível antes de enviar; entregue e cancelado são finais.
var transitions = map[Status][]Status{
	StatusNew:     {StatusPaid, StatusCancelled},
	StatusPaid:    {StatusShipped, StatusCancelled},
	StatusShipped: {StatusDelivered},
}

func CanTransition(from, to Status) bool {
	for _, next := range transitions[from] {
		if next == to {
			return true
		}
	}
	return false
}

var known = map[Status]struct{}{
	StatusNew: {}, StatusPaid: {}, StatusShipped: {}, StatusDelivered: {}, StatusCancelled: {},
}

// ParseStatus aceita só os valores exatos; qualquer outra coisa é inválida.
func ParseStatus(s string) (Status, bool) {
	st := Status(s)
	_, ok := known[st]
	return st, ok
}

func (s Status) IsFinal() bool { return len(transitions[s]) == 0 }
