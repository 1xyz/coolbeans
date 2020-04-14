// +build integration

package integration_test

import (
	"github.com/beanstalkd/go-beanstalk"
	. "github.com/smartystreets/goconvey/convey"
	"math/rand"
	"testing"
	"time"
)

const (
	serverAddr         = "127.0.0.1:11300"
	putTTR             = 10 * time.Second
	reserveTimeout     = 2 * time.Second
	delay              = 5 * time.Second
	msgErrTimeout      = "reserve-with-timeout: timeout"
	msgErrDeadlineSoon = "reserve-with-timeout: deadline soon"
)

func TestProtocol_Put(t *testing.T) {
	Convey("Given a beanstalk service", t, func() {
		c := newConn(t)
		defer c.Close()

		Convey("a producer should be able to put a job", func() {
			id, err := c.Put([]byte("garbanzo beans"), 1, 0, putTTR)
			So(err, ShouldBeNil)
			So(id, ShouldBeGreaterThanOrEqualTo, 0)

			Convey("and should be able to delete", func() {
				err := c.Delete(id)
				So(err, ShouldBeNil)
			})
		})

		Convey("a producer should be able to put a delayed job", func() {
			id, err := c.Put([]byte("garbanzo beans"), 1, delay, putTTR)
			So(err, ShouldBeNil)
			So(id, ShouldBeGreaterThanOrEqualTo, 0)

			Convey("and should be able to delete", func() {
				err := c.Delete(id)
				So(err, ShouldBeNil)
			})
		})

		Convey("a producer should be able to put a job to a specific tube", func() {
			tubeName := randStr(6)
			prodTube := beanstalk.Tube{Conn: c, Name: tubeName}
			defer prodTube.Conn.Close()
			id, _ := prodTube.Put([]byte("garbanzo beans"), 1, 0, putTTR)

			Convey("and should be able to delete", func() {
				err := prodTube.Conn.Delete(id)
				So(err, ShouldBeNil)
			})
		})

		Convey("a producer should be able to put a delayed job to a specific tube", func() {
			tubeName := randStr(6)
			prodTube := beanstalk.Tube{Conn: c, Name: tubeName}
			defer prodTube.Conn.Close()
			id, _ := prodTube.Put([]byte("garbanzo beans"), 1, delay, putTTR)

			Convey("and should be able to delete", func() {
				err := prodTube.Conn.Delete(id)
				So(err, ShouldBeNil)
			})
		})

	})
}

func TestProtocol_Put_Reserve_Delete(t *testing.T) {
	Convey("when a producer puts a job to a specific tube", t, func() {
		tubeName := randStr(6)
		expectedData := "garbanzo beans"
		prodTube := beanstalk.Tube{Conn: newConn(t), Name: tubeName}
		defer prodTube.Conn.Close()
		expectedID, _ := prodTube.Put([]byte(expectedData), 1, 0, putTTR)

		Convey("a consumer should be able to reserve it", func() {
			tubes := beanstalk.NewTubeSet(newConn(t), tubeName)
			defer tubes.Conn.Close()
			actualID, actualData, err := tubes.Reserve(reserveTimeout)
			So(err, ShouldBeNil)
			So(actualID, ShouldEqual, expectedID)
			So(string(actualData), ShouldEqual, expectedData)

			Convey("another connection cannot delete that reserved job", func() {
				conn := newConn(t)
				defer conn.Close()
				err := conn.Delete(actualID)
				So(err, ShouldNotBeNil)
			})

			Convey("only that consumer and should be able to delete it", func() {
				err := tubes.Conn.Delete(actualID)
				So(err, ShouldBeNil)
			})
		})
	})
}

func TestProtocol_Put_Delayed_Reserve_Delete(t *testing.T) {
	Convey("when a producer puts a delayed job to a specific tube", t, func() {
		tubeName := randStr(6)
		expectedData := "garbanzo beans"
		prodTube := beanstalk.Tube{Conn: newConn(t), Name: tubeName}
		defer prodTube.Conn.Close()
		expectedID, _ := prodTube.Put([]byte(expectedData), 1, delay, putTTR)

		Convey("a consumer should not be able to reserve it immediately", func() {
			tubes := beanstalk.NewTubeSet(newConn(t), tubeName)
			defer tubes.Conn.Close()
			_, _, err := tubes.Reserve(delay / 2)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldEqual, msgErrTimeout)
		})

		Convey("a consumer should be able to reserve it when delayed job becomes ready", func() {
			tubes := beanstalk.NewTubeSet(newConn(t), tubeName)
			defer tubes.Conn.Close()
			select {
			case <-time.After(delay + time.Second):
				actualID, _, err := tubes.Reserve(delay / 2)
				So(err, ShouldBeNil)
				So(actualID, ShouldEqual, expectedID)

				Convey("only that consumer and should be able to delete it", func() {
					err := tubes.Conn.Delete(actualID)
					So(err, ShouldBeNil)
				})
			}
		})
	})
}

func TestProtocol_Reserve_TTR(t *testing.T) {
	Convey("given a tube with a single job", t, func() {
		const ttr = 2 * time.Second
		tubeName := randStr(6)
		prodTube := beanstalk.Tube{Conn: newConn(t), Name: tubeName}
		defer prodTube.Conn.Close()
		prodTube.Put([]byte("garbanzo beans"), 1, 0, ttr)

		Convey("and consumer who reserves the job", func() {
			tubes := beanstalk.NewTubeSet(newConn(t), tubeName)
			defer tubes.Conn.Close()
			id, _, err := tubes.Reserve(reserveTimeout)
			defer tubes.Conn.Close()
			So(err, ShouldBeNil)
			So(id, ShouldNotBeEmpty)

			Convey("should be able to reserve the job after the ttr", func() {
				select {
				case <-time.After(3 * time.Second):
					actualID, _, err := tubes.Reserve(ttr)
					So(err, ShouldBeNil)
					So(actualID, ShouldEqual, id)
				}
			})
		})
	})
}

// Behavior of Deadline_Soon
// During the TTR of a reserved job, the last second is kept by the server as a
// safety margin, during which the client will not be made to wait for another
// job. If the client issues a reserve command during the safety margin, or if
// the safety margin arrives while the client is waiting on a reserve command,
// the server will respond with:
//
//    DEADLINE_SOON\r\n
func TestProtocol_Reserve_DeadlineSoon(t *testing.T) {
	Convey("given a tube with a single job", t, func() {
		const ttr = 5 * time.Second
		tubeName := randStr(6)
		prodTube := beanstalk.Tube{Conn: newConn(t), Name: tubeName}
		defer prodTube.Conn.Close()
		prodTube.Put([]byte("garbanzo beans"), 1, 0, ttr)

		Convey("and consumer who reserves the job", func() {
			tubes := beanstalk.NewTubeSet(newConn(t), tubeName)
			defer tubes.Conn.Close()
			id, _, err := tubes.Reserve(reserveTimeout)
			So(err, ShouldBeNil)
			So(id, ShouldNotBeEmpty)

			Convey("receives a deadline_soon for the next reserve in the last second of the ttr", func() {
				_, _, err := tubes.Reserve(ttr)
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldEqual, msgErrDeadlineSoon)
			})
		})
	})
}

func TestProtocol_Reserve_Timeout(t *testing.T) {
	Convey("when a consumer reserves a tube with no jobs", t, func() {
		tubeName := randStr(6)
		tubes := beanstalk.NewTubeSet(newConn(t), tubeName)
		defer tubes.Conn.Close()

		Convey("the request times out", func() {
			errCh := make(chan error)
			go func() {
				_, _, err := tubes.Reserve(2 * time.Second)
				errCh <- err
				close(errCh)
			}()

			select {
			case <-time.After(10 * time.Second):
				t.Fatalf("timeout failed")
			case err := <-errCh:
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldEqual, msgErrTimeout)
			}
		})
	})

	Convey("when a consumer reserves a tube with no jobs with a timeout of zero", t, func() {
		tubeName := randStr(6)
		tubes := beanstalk.NewTubeSet(newConn(t), tubeName)
		defer tubes.Conn.Close()

		Convey("the request times out immediately", func() {
			errCh := make(chan error)
			go func() {
				_, _, err := tubes.Reserve(0 * time.Second)
				errCh <- err
				close(errCh)
			}()

			select {
			case <-time.After(2 * time.Second):
				t.Fatalf("timeout failed")
			case err := <-errCh:
				So(err, ShouldNotBeNil)
				So(err.Error(), ShouldEqual, msgErrTimeout)
			}
		})
	})
}

func newConn(t *testing.T) *beanstalk.Conn {
	c, err := beanstalk.Dial("tcp", serverAddr)
	if err != nil {
		t.Fatalf("error dial beanstalkd %v", err)
	}

	return c
}

const charset = "abcdefghijklmnopqrstuvwxyz" +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

func randStr(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}
