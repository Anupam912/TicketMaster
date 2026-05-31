package queue

// ExpiryPartitionIndex returns the stable partition for a booking ID.
// Must stay in sync with the Lua hash in claimExpiryScript.
func ExpiryPartitionIndex(member string, partitionCount int) int {
	if partitionCount <= 1 {
		return 0
	}

	const modulus int64 = 2147483647
	var h int64 = 0
	for i := 0; i < len(member); i++ {
		h = (h*31 + int64(member[i])) % modulus
	}
	return int(h % int64(partitionCount))
}
