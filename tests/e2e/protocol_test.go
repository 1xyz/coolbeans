// +build integration

package e2e_test

import (
	"fmt"
	"github.com/beanstalkd/go-beanstalk"
	. "github.com/smartystreets/goconvey/convey"
	"math/rand"
	"testing"
	"time"
)

const (
	serverAddr            = "127.0.0.1:11300"
	putTTR                = 10 * time.Second
	reserveTimeout        = 2 * time.Second
	delay                 = 5 * time.Second
	msgErrTimeout         = "reserve-with-timeout: timeout"
	msgErrDeadlineSoon    = "reserve-with-timeout: deadline soon"
	msgErrBuryNotFound    = "bury: not found"
	msgErrKickNotFound    = "kick-job: not found"
	msgErrTouchNotFound   = "touch: not found"
	msgErrReleaseNotFound = "release: not found"
	msgErrPeekNotFound    = "peek: not found"
	msgErrNotFound        = "not found"
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

func TestProtocol_Bury_Kick(t *testing.T) {
	Convey("when a producer put a job to a specific tube", t, func() {
		tubeName := randStr(6)
		prodTube := beanstalk.Tube{Conn: newConn(t), Name: tubeName}
		defer prodTube.Conn.Close()
		jobId, _ := prodTube.Put([]byte("garbanzo beans"), 1, 0, putTTR)

		Convey("which is reserved by a consumer", func() {
			tubes := beanstalk.NewTubeSet(newConn(t), tubeName)
			defer tubes.Conn.Close()
			id, _, _ := tubes.Reserve(reserveTimeout)

			Convey("another consumer cannot bury that reserved job", func() {
				conn := newConn(t)
				defer conn.Close()
				err := conn.Bury(id, 3)
				So(err.Error(), ShouldEqual, msgErrBuryNotFound)
			})

			Convey("only that consumer can bury that job", func() {
				err := tubes.Conn.Bury(id, 3)
				So(err, ShouldBeNil)

				Convey("and anyone can kick that job", func() {
					conn := newConn(t)
					defer conn.Close()
					err := conn.KickJob(id)
					So(err, ShouldBeNil)
				})

				Convey("and anyone kick that job with a bound", func() {
					tube := beanstalk.Tube{
						Conn: newConn(t),
						Name: tubeName,
					}
					defer tube.Conn.Close()
					n, err := tube.Kick(3)
					So(err, ShouldBeNil)
					So(n, ShouldEqual, 1)
				})
			})
		})

		Convey("kicking an unreserved job leads to nothing", func() {
			conn := newConn(t)
			defer conn.Close()
			err := conn.KickJob(jobId)
			So(err.Error(), ShouldEqual, msgErrKickNotFound)
		})
	})
}

func TestProtocol_Touch(t *testing.T) {
	Convey("when a producer put a job to a specific tube", t, func() {
		tubeName := randStr(6)
		prodTube := beanstalk.Tube{Conn: newConn(t), Name: tubeName}
		defer prodTube.Conn.Close()
		jobID, _ := prodTube.Put([]byte("garbanzo beans"), 1, 0, putTTR)

		Convey("which is reserved by a consumer", func() {
			tubes := beanstalk.NewTubeSet(newConn(t), tubeName)
			defer tubes.Conn.Close()
			id, _, _ := tubes.Reserve(reserveTimeout)

			Convey("another consumer cannot touch that reserved job", func() {
				conn := newConn(t)
				defer conn.Close()
				err := conn.Touch(id)
				So(err.Error(), ShouldEqual, msgErrTouchNotFound)
			})

			Convey("only that consumer can touch that job", func() {
				err := tubes.Conn.Touch(id)
				So(err, ShouldBeNil)
			})
		})

		Convey("touch an unreserved job leads to nothing", func() {
			conn := newConn(t)
			defer conn.Close()
			err := conn.Touch(jobID)
			So(err.Error(), ShouldEqual, msgErrTouchNotFound)
		})
	})
}

func TestProtocol_Release(t *testing.T) {
	Convey("when a producer put a job to a specific tube", t, func() {
		tubeName := randStr(6)
		prodTube := beanstalk.Tube{Conn: newConn(t), Name: tubeName}
		defer prodTube.Conn.Close()
		jobID, _ := prodTube.Put([]byte("garbanzo beans"), 1, 0, putTTR)

		Convey("which is reserved by a consumer", func() {
			tubes := beanstalk.NewTubeSet(newConn(t), tubeName)
			defer tubes.Conn.Close()
			id, _, _ := tubes.Reserve(reserveTimeout)

			Convey("another consumer cannot release that reserved job", func() {
				conn := newConn(t)
				defer conn.Close()
				err := conn.Release(id, 10, 0)
				So(err.Error(), ShouldEqual, msgErrReleaseNotFound)
			})

			Convey("only that consumer can release that job", func() {
				err := tubes.Conn.Release(id, 10, 0)
				So(err, ShouldBeNil)
			})

			Convey("only that consumer can release that job with a delay", func() {
				err := tubes.Conn.Release(id, 10, 10)
				So(err, ShouldBeNil)
			})
		})

		Convey("touch an unreserved job leads to nothing", func() {
			conn := newConn(t)
			defer conn.Close()
			err := conn.Release(jobID, 10, 0)
			So(err.Error(), ShouldEqual, msgErrReleaseNotFound)
		})
	})
}

func TestProtocol_Peek(t *testing.T) {
	Convey("when a producer put a job to a specific tube", t, func() {
		tubeName := randStr(6)
		prodTube := beanstalk.Tube{Conn: newConn(t), Name: tubeName}
		defer prodTube.Conn.Close()
		bodyIn := []byte("garbanzo beans")
		jobID, _ := prodTube.Put(bodyIn, 1, 0, putTTR)

		Convey("the job can be peeked by anyone", func() {
			c := newConn(t)
			defer c.Close()
			bodyOut, err := c.Peek(jobID)
			So(err, ShouldBeNil)
			So(bodyOut, ShouldResemble, bodyIn)
		})
	})

	Convey("Peek an unknown job results in not_found", t, func() {
		c := newConn(t)
		defer c.Close()
		_, err := c.Peek(1234)
		So(err.Error(), ShouldEqual, msgErrPeekNotFound)
	})
}

type jobReq struct {
	id   uint64
	body []byte
}

func TestProtocol_PeekReady(t *testing.T) {
	Convey("when a producer put jobs with different priorities to a specific tube", t, func() {
		pt := newRandTube(t)
		defer closeTube(t, pt)

		n := 3
		jobsIn := make([]jobReq, n)
		for i := 0; i < n; i++ {
			jobsIn[i].body = []byte(fmt.Sprintf("hello_%d", i))
			jobsIn[i].id, _ = pt.Put(jobsIn[i].body, uint32(n-i), 0, putTTR)
		}

		Convey("Peeking the ready tube returns the highest pri job", func() {
			ct := newTube(t, pt.Name)
			defer closeTube(t, ct)

			idOut, bodyOut, err := ct.PeekReady()
			So(err, ShouldBeNil)
			So(idOut, ShouldEqual, jobsIn[2].id)
			So(bodyOut, ShouldResemble, jobsIn[2].body)
		})
	})

	Convey("Peeking an empty ready tube returns not-found", t, func() {
		ct := newRandTube(t)
		defer closeTube(t, ct)

		_, _, err := ct.PeekReady()
		So(err.Error(), ShouldContainSubstring, msgErrNotFound)
	})
}

func TestProtocol_PeekDelayed(t *testing.T) {
	Convey("when a producer put jobs with different delays to a specific tube", t, func() {
		pt := newRandTube(t)
		defer closeTube(t, pt)

		n := 3
		jobsIn := make([]jobReq, n)
		for i := 0; i < n; i++ {
			delay := time.Duration(10+n-i) * time.Second
			jobsIn[i].body = []byte(fmt.Sprintf("hello_%d", i))
			jobsIn[i].id, _ = pt.Put(jobsIn[i].body, 0, delay, putTTR)
		}

		Convey("Peeking the delayed tube returns the lowest delay job", func() {
			ct := newTube(t, pt.Name)
			defer closeTube(t, ct)

			idOut, bodyOut, err := ct.PeekDelayed()
			So(err, ShouldBeNil)
			So(idOut, ShouldEqual, jobsIn[2].id)
			So(bodyOut, ShouldResemble, jobsIn[2].body)
		})
	})

	Convey("Peeking an empty delayed tube returns not-found", t, func() {
		ct := newRandTube(t)
		defer closeTube(t, ct)

		_, _, err := ct.PeekDelayed()
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, msgErrNotFound)
	})
}

func newBuriedJob(t *testing.T, tube *beanstalk.Tube, body []byte, pri uint32, delay time.Duration) uint64 {
	jobId, err := tube.Put(body, pri, delay, putTTR)
	if err != nil {
		t.Fatalf("newBuriedJob: tube.Put err = %v", err)
	}

	tubes := newTubeSet(t, tube.Name)
	defer closeTubeSet(t, tubes)
	id, _, err := tubes.Reserve(reserveTimeout)
	if err != nil {
		t.Fatalf("newBuriedJob: tubes.Reserve err = %v", err)
	}
	if id != jobId {
		t.Fatalf("newBuriedJob: tubes.Reserve found another job in the queue id = %v expected id = %v",
			jobId, id)
	}

	if err := tubes.Conn.Bury(id, 254); err != nil {
		t.Fatalf("newBuriedJob: conn.Bury err = %v", err)
	}

	return jobId
}

func TestProtocol_PeekBuried(t *testing.T) {
	Convey("when a producer buries jobs to a specific tube", t, func() {
		pt := newRandTube(t)
		defer closeTube(t, pt)

		n := 3
		jobsIn := make([]jobReq, n)
		for i := 0; i < n; i++ {
			jobsIn[i].body = []byte(fmt.Sprintf("hello_%d", i))
			jobsIn[i].id = newBuriedJob(t, pt, jobsIn[i].body, 0, 0)
		}

		Convey("Peeking the buried tube returns the earliest buried job", func() {
			ct := newTube(t, pt.Name)
			defer closeTube(t, ct)

			idOut, bodyOut, err := ct.PeekBuried()
			So(err, ShouldBeNil)
			So(idOut, ShouldEqual, jobsIn[0].id)
			So(bodyOut, ShouldResemble, jobsIn[0].body)
		})
	})

	Convey("PeekBuried an empty tube returns not-found", t, func() {
		ct := newRandTube(t)
		defer closeTube(t, ct)

		_, _, err := ct.PeekBuried()
		So(err, ShouldNotBeNil)
		So(err.Error(), ShouldContainSubstring, msgErrNotFound)
	})
}

func newTubeSet(t *testing.T, tubeName ...string) *beanstalk.TubeSet {
	return beanstalk.NewTubeSet(newConn(t), tubeName...)
}

func newTube(t *testing.T, tubeName string) *beanstalk.Tube {
	return &beanstalk.Tube{Conn: newConn(t), Name: tubeName}
}

func newRandTube(t *testing.T) *beanstalk.Tube {
	tubeName := randStr(6)
	return newTube(t, tubeName)
}

func closeTubeSet(t *testing.T, tubeSet *beanstalk.TubeSet) {
	closeConn(t, tubeSet.Conn)
}

func closeTube(t *testing.T, tube *beanstalk.Tube) {
	closeConn(t, tube.Conn)
}

func closeConn(t *testing.T, c *beanstalk.Conn) {
	if c != nil {
		if err := c.Close(); err != nil {
			t.Logf("closeTube: c.close() err=%v", err)
		}
	}
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
