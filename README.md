# go-log-rotation
Go library to log messages to files with hourly rotation

This library is mainly designed to archive a stream of string chat messages to log files.

It is written with the following assumptions:

* The logger may be used with multiple goroutines.
* Messages arrive with timestamps that are MOSTLY in increasing order.
  * It is possible that the timestamps are slightly out of order, but they should be uncommon and such messages are not very far from their correct places. (maximum a few seconds difference)
* Messages timestamps may be logged slightly out of order in the same log file
* Messages will be logged in log files for the correct hour buckets (e.g. one at :59:59.9999 and another at :00.00.0001 will always be logged in different files)
* Messages will be flushed to file every N (configurable) seconds. The last few seconds of chat messages in the buffer may be lost in case of ungraceful shutdown (power failure, etc).
* Each message is appended as a new line, and the file will always end with an empty line.
