## wikiPolite: A Simple Web Scanner

wikiPolite is a robust web scanner written in Go that sends HTTP GET requests to a list of URLs concurrently. The scanner abides by the rules outlined in each site's `robots.txt` file and upholds domain-specific rate limits to ensure polite and efficient scanning.

The architecture of the scanner comprises a primary control ('Main') and a series of domain-dedicated 'Workers'

### Main

Main is responsible for reading the tasks from an input file, handle connection to database, and assigning them to appropriate Workers. Main keeps track of the expected runtime for the entire scan, initiates all workers, and listens to a result channel. Upon receiving results from the result channel, Main ***keeps logging*** to database in a program-dedicated collection.

### Workers

Each Worker is dedicated to a specific domain and listens to a task input channel. When a Worker is created, it calculates an appropriate crawl delay by dividing the expected runtime by the number of tasks it has to handle. It then sleeps for a random duration, between 0 and the calculated crawl delay, introducing some ***randomness*** to the scanning process. 

Each Worker fetches the `robots.txt` files for both HTTP and HTTPS, and subsequently handles the tasks. A task involves:

* Check if the path is scannable given the scheme, and capture any errors
* Perform a DNS lookup, and capture any errors
* Establish a TCP connection, and capture any errors
* Perform a TLS handshake (for HTTPS), and capture any errors
* Issue request with a formulated user-agent, and capture any errors
* Set up redirect chain logging
* Send the request, and capture any errors
* If the current scheme is HTTP and the response status code isn't `200 OK`, the Worker ***retries*** with HTTPS and makes a note.
* Upon task completion, the Worker sends the results back to Main.

### Program Flow

Initialization: Main loads tasks from an input file, prepare database for output, and assigns tasks to Workers based on the domain.

Task Execution: The Workers complete their tasks within the expected runtime, and send results back to Main.

Result Handling: Main writes results from the result channel ***MongoDB***

### Output Example

```
[
  {
    _id: ObjectId("64a75ca83194a40fedbfd33b"),
    domain: 'news.cincinnati.com',
    url: 'http://news.cincinnati.com/article/20120521/SPT01/305210101/Exclusive-Crosstown-Shootout-set-U-S-Bank-Arena?odyssey=mod%7cmostview',
    ip: '159.54.247.168',
    autoretryhttps: false,
    statuscode: 200,
    redirectchain: [
      '301 http://www.cincinnati.com',
      '301 https://www.cincinnati.com/'
    ],
    err: ''
  },
  {
    _id: ObjectId("64a75ca83194a40fedbfd33c"),
    domain: 'docs.lib.noaa.gov',
    url: 'https://docs.lib.noaa.gov/rescue/mwr/054/mwr-054-07-0312.pdf',
    ip: '',
    autoretryhttps: false,
    statuscode: 0,
    redirectchain: null,
    err: 'DNS error: lookup docs.lib.noaa.gov: no such host'
  },
  {
    _id: ObjectId("64a75ca83194a40fedbfd33d"),
    domain: 'www.saraparetsky.com',
    url: 'http://www.saraparetsky.com',
    ip: '141.193.213.10',
    autoretryhttps: false,
    statuscode: 200,
    redirectchain: [ '301 https://www.saraparetsky.com/' ],
    err: ''
  }
]
```