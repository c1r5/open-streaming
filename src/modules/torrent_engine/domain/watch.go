package domain

// MovieWatchID and EpisodeWatchID share /stream/watch/{id} without colliding.
// Movies use even numbers. Episodes use odd numbers.
func MovieWatchID(id uint) uint { return id * 2 }

func EpisodeWatchID(id uint) uint { return id*2 + 1 }

func ParseWatchID(id uint) (episode bool, real uint) {
	if id%2 == 1 {
		return true, (id - 1) / 2
	}
	return false, id / 2
}
