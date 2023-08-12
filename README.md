## wikiPolite: A Simple Web Scanner

wikiPolite is a robust web scanner written in Go that sends HTTP GET requests to a list of URLs concurrently. The scanner abides by the rules outlined in each site's `robots.txt` file and upholds domain-specific rate limits to ensure polite and efficient scanning.

The architecture of the scanner comprises a primary control ('Main') and a series of domain-dedicated 'Workers'

### Main

Main is responsible for reading the tasks from an input file, handle connection to database, and assigning them to appropriate Workers. Main keeps track of the expected runtime for the entire scan, initiates all workers, and listens to a result channel. Upon receiving results from the result channel, Main ***keeps logging*** to database in a program-dedicated collection.

### Workers

Each Worker is dedicated to a specific domain and listens to a task input channel. When a Worker is created, it calculates an appropriate crawl delay by dividing the expected runtime by the number of tasks it has to handle. It then sleeps for a random duration, between 0 and the calculated crawl delay, introducing some ***randomness*** to the scanning process. 

Each Worker fetches the `robots.txt` files for both HTTP and HTTPS, and subsequently handles the tasks. A task involves:

* Check if the path is scan-able given the scheme, and capture any errors
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
    _id: ObjectId("64d7f248efc44cbde6332536"),
    sourceurl: 'B',
    domain: 'www.microsoft.com',
    url: 'https://www.microsoft.com/en-us',
    ip: '23.216.81.152',
    redirectchain: [],
    resp: {
      statuscode: 200,
      header: {
        'Ms-Commit-Id': [ 'b073854' ],
        'X-Vhost': [ 'publish_microsoft_s' ],
        'Cache-Control': [ 'max-age=0,s-maxage=86400' ],
        'Content-Type': [ 'text/html;charset=utf-8' ],
        'X-Dispatcher': [ 'dispatcher2westus2' ],
        'X-Rtag': [ 'AEM_PROD_Marketing' ],
        'Strict-Transport-Security': [ 'max-age=31536000' ],
        Tls_version: [ 'tls1.3' ],
        'Ms-Cv': [ 'CASMicrosoftCV175f222a.0' ],
        'X-Edgeconnect-Origin-Mex-Latency': [ '423' ],
        'X-Edgeconnect-Midmile-Rtt': [ '0' ],
        Etag: [ '"6558dbf2a5466fb03d14a682751ec447-gzip"' ],
        'X-Version': [ '2023.809.100912.0003033343' ],
        Date: [ 'Sat, 12 Aug 2023 20:57:44 GMT' ],
        'Set-Cookie': [
          'AEMDC=westus2; path=/; secure; SameSite=None',
          'akacd_OneRF=1699649863~rv=99~id=72f704ac1115329fda7d5d77008ffdf9; path=/; Expires=Fri, 10 Nov 2023 20:57:43 GMT; Secure; SameSite=None',
          'akacd_OneRF=1699649863~rv=99~id=72f704ac1115329fda7d5d77008ffdf9; path=/; Expires=Fri, 10 Nov 2023 20:57:43 GMT; Secure; SameSite=None'
        ],
        'Ms-Cv-Esi': [ 'CASMicrosoftCV175f222a.0' ],
        Vary: [ 'Accept-Encoding' ],
        'X-Frame-Options': [ 'SAMEORIGIN' ],
        'X-Content-Type-Options': [ 'nosniff' ]
      },
      etag: '6558dbf2a5466fb03d14a682751ec447-gzip',
      eselftag: 'da39a3ee5e6b4b0d3255bfef95601890afd80709'
    },
    err: '',
    autoretryhttps: {
      retried: false,
      redirectchain: null,
      resp: { statuscode: 0, header: null, etag: '', eselftag: '' },
      err: ''
    }
  },
  {
    _id: ObjectId("64d7f249efc44cbde6332537"),
    sourceurl: 'B',
    domain: 'www.google.com',
    url: 'http://www.google.com/maps/',
    ip: '',
    redirectchain: null,
    resp: { statuscode: 0, header: null, etag: '', eselftag: '' },
    err: 'path /maps/ not allowd for http',
    autoretryhttps: {
      retried: true,
      redirectchain: null,
      resp: { statuscode: 0, header: null, etag: '', eselftag: '' },
      err: 'path /maps/ not allowd for https'
    }
  }
]
```