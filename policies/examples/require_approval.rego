package example

default is_allowed = false

# Allow if not sensitive, or if approved
is_allowed {
	not input.sensitive
}

is_allowed {
	input.sensitive
	input.approved == true
}
