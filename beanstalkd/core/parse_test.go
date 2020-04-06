package core

import (
	"bytes"
	. "github.com/smartystreets/goconvey/convey"
	"io"
	"testing"
)

func TestDiscard(t *testing.T) {
	Convey("when Discard is called", t, func() {
		Convey("with an empty buffer", func() {
			callDiscard := func(s string) ([]byte, error) {
				return Discard(bytes.NewReader([]byte(s)), make([]byte, 0))
			}

			Convey("and an input stream with a delimiter", func() {
				rt, err := callDiscard("peek example_tube\r\n peek a boo")

				Convey("all bytes before delim are discarded", func() {
					So(rt, ShouldResemble, []byte(" peek a boo"))
					So(err, ShouldBeNil)
				})
			})

			Convey("and an input stream ending with a delimiter", func() {
				rt, err := callDiscard("peek example_tube\r\n")

				Convey("all bytes before delim are discarded", func() {
					So(rt, ShouldResemble, []byte(""))
					So(err, ShouldBeNil)
				})
			})

			Convey("and an input stream with no delimiter", func() {
				rt, err := callDiscard("peek example_tube ....")

				Convey("all bytes are discarded until EOF", func() {
					So(rt, ShouldBeNil)
					So(err, ShouldEqual, io.EOF)
				})
			})
		})

		Convey("with a non-empty buffer", func() {
			callDiscard := func(s string, buf string) ([]byte, error) {
				return Discard(bytes.NewReader([]byte(s)), []byte(buf))
			}

			Convey("which has a delimiter", func() {
				buf := "peek \r\n hello"

				Convey("and an input stream", func() {
					rt, err := callDiscard("peek example_tube\r\n peek a boo", buf)

					Convey("all bytes before delim are discarded", func() {
						So(rt, ShouldResemble, []byte(" hello"))
						So(err, ShouldBeNil)
					})
				})
			})

			Convey("which has no delimiter", func() {
				buf := "peek  hello"

				Convey("and an input stream with a delimiter", func() {
					rt, err := callDiscard("peek example_tube\r\n peek a boo", buf)

					Convey("all bytes before delim are discarded", func() {
						So(string(rt), ShouldResemble, " peek a boo")
						So(err, ShouldBeNil)
					})
				})

				Convey("and an input stream with no delimiter", func() {
					rt, err := callDiscard("peek example_tube  peek a boo", buf)

					Convey("all bytes before delim are discarded", func() {
						So(rt, ShouldBeNil)
						So(err, ShouldEqual, io.EOF)
					})
				})
			})

			Convey("which ends with carriage return", func() {
				buf := "peek \r"

				Convey("and an input stream starting with newline", func() {
					rt, err := callDiscard("\npeek example_tube\r\n peek a boo", buf)

					Convey("all bytes before delim are discarded", func() {
						So(string(rt), ShouldResemble, "peek example_tube\r\n peek a boo")
						So(err, ShouldBeNil)
					})
				})
			})
		})
	})
}

func TestScan(t *testing.T) {
	Convey("when Scan is called", t, func() {
		Convey("with an empty buffer and a limit", func() {
			callScan := func(s string) ([]byte, []byte, error) {
				return Scan(bytes.NewReader([]byte(s)), make([]byte, 0), 30)
			}

			Convey("and an input stream ending with the delimiter", func() {
				lt, rt, err := callScan("peek example_tube\r\n")

				Convey("scans the stream across the delimiter", func() {
					So(string(lt), ShouldResemble, "peek example_tube")
					So(rt, ShouldResemble, []byte(""))
					So(err, ShouldBeNil)
				})
			})

			Convey("and an input stream containing the delimiter", func() {
				lt, rt, err := callScan("peek example_tube\r\n hello ")

				Convey("scans the stream across the delimiter", func() {
					So(string(lt), ShouldResemble, "peek example_tube")
					So(string(rt), ShouldResemble, " hello ")
					So(err, ShouldBeNil)
				})
			})

			Convey("and an input stream containing just the delimiter", func() {
				lt, rt, err := callScan("\r\n")

				Convey("scans the stream as expected", func() {
					So(lt, ShouldBeEmpty)
					So(rt, ShouldBeEmpty)
					So(err, ShouldBeNil)
				})
			})

			Convey("and an input stream not containing a delimiter within a limit", func() {
				lt, rt, err := callScan("peek example")

				Convey("scans results in a EOF error", func() {
					So(lt, ShouldBeNil)
					So(rt, ShouldBeNil)
					So(err, ShouldEqual, io.EOF)
				})
			})

			Convey("and an input stream with no delimiter and size exceeding the limit", func() {
				lt, rt, err := callScan("peek a large line with no end in site hello")

				Convey("scan results in ErrDelimiterMissing error", func() {
					So(lt, ShouldBeNil)
					So(rt, ShouldBeNil)
					So(err, ShouldEqual, ErrDelimiterMissing)
				})
			})

			Convey("and an input stream with a delimiter and size exceeding the limit", func() {
				lt, rt, err := callScan("peek a large line\r\n with no end in site hello")

				Convey("scan the stream as expected upto only limit bytes", func() {
					So(string(lt), ShouldEqual, "peek a large line")
					So(string(rt), ShouldEqual, " with no en")
					So(err, ShouldBeNil)
				})
			})

			Convey("and an input stream with a delimiter at the limit boundary", func() {
				lt, rt, err := callScan("peek a large line with no en\r\n in site hello")

				Convey("scan the stream as expected upto only limit bytes", func() {
					So(string(lt), ShouldEqual, "peek a large line with no en")
					So(string(rt), ShouldEqual, "")
					So(err, ShouldBeNil)
				})
			})
		})

		Convey("with a fixed limit", func() {
			callScan := func(s string, buf string) ([]byte, []byte, error) {
				return Scan(bytes.NewReader([]byte(s)), []byte(buf), 30)
			}

			Convey("and a non-empty buffer with a part of the delimiter", func() {
				buf := "peek \r"

				Convey("and an input stream with the second half of the delimiter", func() {
					lt, rt, err := callScan("\nexample_tube\r\nhello", buf)

					Convey("results in a success", func() {
						So(string(lt), ShouldResemble, "peek ")
						So(string(rt), ShouldResemble, "example_tube\r\nhello")
						So(err, ShouldBeNil)
					})
				})

				Convey("and an input stream with only the second half of the delimiter", func() {
					lt, rt, err := callScan("\n", buf)

					Convey("results in a success", func() {
						So(string(lt), ShouldResemble, "peek ")
						So(string(rt), ShouldResemble, "")
						So(err, ShouldBeNil)
					})
				})
			})

			Convey("and a non-empty buffer with no delimiter", func() {
				buf := "peek "

				Convey("and an input stream with a delimiter", func() {
					lt, rt, err := callScan("example_tube\r\nhello", buf)

					Convey("results in a success", func() {
						So(string(lt), ShouldResemble, "peek example_tube")
						So(rt, ShouldResemble, []byte("hello"))
						So(err, ShouldBeNil)
					})
				})

				Convey("and an input stream with no delimiter exceeding the limit", func() {
					lt, rt, err := callScan("example_tube hello word this crazy cool", buf)

					Convey("results in a ErrDelimiterMissing error", func() {
						So(lt, ShouldBeNil)
						So(rt, ShouldBeNil)
						So(err, ShouldEqual, ErrDelimiterMissing)
					})
				})
			})

			Convey("and a non-empty buffer with  delimiter", func() {
				buf := "peek queue \r\n"

				Convey("and an input stream", func() {
					lt, rt, err := callScan("example_tube\r\nhello", buf)

					Convey("results in a success", func() {
						So(string(lt), ShouldResemble, "peek queue ")
						So(string(rt), ShouldResemble, "")
						So(err, ShouldBeNil)
					})
				})

				Convey("and an empty input stream", func() {
					lt, rt, err := callScan("l", buf)

					Convey("results in a success", func() {
						So(string(lt), ShouldResemble, "peek queue ")
						So(rt, ShouldBeEmpty)
						So(err, ShouldBeNil)
					})
				})
			})
		})
	})
}
