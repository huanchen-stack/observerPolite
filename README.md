## wikiPolite: A Simple Web Scanner

wikiPolite is a robust web scanner written in Go that sends HTTP GET requests to a list of URLs concurrently. The scanner abides by the rules outlined in each site's `robots.txt` file and upholds domain-specific rate limits to ensure polite and efficient scanning.

The architecture of the scanner comprises a primary control ('Main') and a series of domain-dedicated 'Workers'

### Main

Main is responsible for reading the tasks from an input file and assigning them to appropriate Workers. Main keeps track of the expected runtime for the entire scan, initiates all workers, and listens to a result channel. It ensures all tasks have been processed before it writes the results to an output file and terminates the program.

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

Initialization: Main loads tasks from an input file and assigns tasks to Workers based on the domain.

Task Execution: The Workers complete their tasks within the expected runtime, and send results back to Main.

Result Handling: Main writes results from the result channel to an output `.txt` file.  ***Next Step: Log Results to a Database***

### Output Example

```
[
    {
        "Domain": "www.microsoft.com",
        "URL": "http://www.microsoft.com/",
        "IP": "104.73.1.162",
        "AutoRetryHTTPS": false,
        "StatusCode": 200,
        "RedirectChain": [
            "302 https://www.microsoft.com/en-us/"
        ],
        "Err": ""
    },
    {
        "Domain": "example.com",
        "URL": "https://example.com/example",
        "IP": "93.184.216.34",
        "AutoRetryHTTPS": true,
        "StatusCode": 404,
        "RedirectChain": null,
        "Err": ""
    },
    {
        "Domain": "www.microsoft.com",
        "URL": "https://www.microsoft.com/en-us/windows/si/matrix.html",
        "IP": "104.73.1.162",
        "AutoRetryHTTPS": true,
        "StatusCode": 404,
        "RedirectChain": null,
        "Err": "path /en-us/windows/si/matrix.html not allowd for https"
    }
]
```