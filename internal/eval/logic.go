package eval

func truth(condition bool) Truth {
	if condition {
		return True
	}
	return False
}

func negate(value Truth) Truth {
	switch value {
	case True:
		return False
	case False:
		return True
	default:
		return Unknown
	}
}

func and(left, right Truth) Truth {
	if left == False || right == False {
		return False
	}
	if left == Unknown || right == Unknown {
		return Unknown
	}
	return True
}

func or(left, right Truth) Truth {
	if left == True || right == True {
		return True
	}
	if left == Unknown || right == Unknown {
		return Unknown
	}
	return False
}
