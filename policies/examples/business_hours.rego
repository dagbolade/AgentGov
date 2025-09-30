package example

default is_allowed = false

# Only allow requests between 9am and 5pm (inclusive)
# Expects input.hour (integer, 0-23)
is_allowed {
	input.hour >= 9
	input.hour <= 17
}
