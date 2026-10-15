// Module d2std: la biblioteca estandar minima para la Dreamcast.
//
// Por que existe: en sh-elf NO hay biblioteca estandar de Go (libgodc no la trae). Cada paquete
// de aca se escribe una sola vez y se compila de dos maneras:
//
//   - en el host: como este modulo, contra el go normal. Sirve para la prueba diferencial
//     (misma entrada por nuestra implementacion y por la de Go, comparacion exacta).
//   - en la Dreamcast: con sh-elf-gccgo. Los paquetes se importan entre si como "d2std/xxx" y
//     el script de build los compila a .gox en el orden de dependencias, de modo que tambien
//     resuelven como "xxx" cuando el motor pide "fmt" a secas.
//
// Regla de la casa: estos paquetes NO pueden importar la biblioteca estandar de verdad (ni
// fmt, ni strings, ni reflect ni unsafe mas alla de lo imprescindible). Si hace falta algo, se
// escribe aca.
module d2std

go 1.21
