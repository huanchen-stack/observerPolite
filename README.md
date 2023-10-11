#### Code Structure
* COMMON
  * config.go: store all hyper parameters
  * heap.go: go lang doesn't have a default heap
  * type.go: declare most of structs
  * util.go: mostly io helper functions
* DEPRECATED (*skip for now*)
* HEARTBEAT (currently deprecated) (*skip for now*)
* MAIN
  * main.go: all go routines are started here
* MONGODB
  * mongodb.go: handle read/write scan results
  * robotsdb.go: handle read/write robots.txt
* RETRYMANAGER
  * retrymanager.go: start a retry worker periodically
* WORKER
  * generalworker.go: worker that does all the scans 

#### GeneralWorker Design

#### Robotstxt Fetch&Log Design
* Use temoto/robotstxt for parsing and path testing
* tomoto/robotstxt's (path) Group object cannot be logged to database.
  This is because the group object stores all path (allow/disallow) regex as a private variable.
* `robots.txt` in string format is stored in DB, this can be converted to robotstxt's Group object
* All DB writes are in batches, the batch not yet written acts like a natural cache.
#### Retry Design

