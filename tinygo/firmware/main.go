// Command firmware is cameo on a microcontroller: it runs one script
// compiled into it and prints what the script prints. `make
// tinygo-baremetal` builds it for TinyGo's emulated RISC-V board and runs it
// in QEMU. A real board needs far more RAM than an ESP32 has; see the
// TinyGo section of the README.
package main

import (
	"os"

	"github.com/cameo-lang/cameo/src/cameo"
)

const script = `
class Point
  int :x, :y
  def +(o) = Point.new(x: x + o.x, y: y + o.y)
end

p Point.new(x: 1, y: 2) + Point.new(x: 3, y: 4)
p (1..10).map { |i| i * i }.select(&:even?)
`

func main() {
	if err := cameo.Run("firmware.cameo", script, os.Stdout, os.Stderr); err != nil {
		println(err.Error())
	}
}
