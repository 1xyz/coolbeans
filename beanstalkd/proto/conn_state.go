package proto

type ConnState int

const (
	// Conn waiting for a command from the client
	WantCommand ConnState = iota

	// Conn ready to dispatch
	ReadyToProcess

	// Conn waiting for job data from the client
	WantData

	// Conn is sending a reservation job data to the client
	SendJob

	// Conn is sending a  response to the client
	SendWord

	// Conn is waiting for an available job reservation
	WaitReservation

	// Conn is discarding job data (due to an error)
	BitBucket

	// Conn is closing & cleaning up
	Close

	// Conn is discarding data until it gets to \r\n
	WantEndLine

	// The connection is cleaned up and shutdown
	Stopped
)
