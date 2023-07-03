## wikiPolite: A Simple Web Scanner

wikiPolite is a web scanner that issues HTTP GET requests to a list of URLs. The scanner utilizes Go routines to perform concurrent scans and is designed to be efficient and polite, adhering to rules set in `robots.txt` files and maintaining a domain-specific rate limit. 

The scanner consists of a main Task Manager and domain-dedicated Workers, with each category having different responsibilities.

### Task Manager

The Task Manager reads tasks from an input file, fetches and evaluates `robots.txt` for each domain. URLs disallowed by `robots.txt` are removed from the task list, while the rest are passed to appropriate Workers. The Task Manager keeps track of an expected runtime for the entire scan and starts all workers. It listens to a result channel, writes results to an output file, and ensures all tasks are processed before termination.

### Workers

Each Worker is dedicated to a specific domain and listens to a task input channel. The Worker wakes up periodically, completing its assigned tasks within the expected runtime. The scan involves DNS resolution, establishing a TLS connection (for HTTPS), and sending a GET request. Any failures and redirects during the process are recorded. Upon completing a task, the Worker writes results, including error messages, redirect chain and time of scan, to the result channel in the Task Manager.

### Program Flow

1. Initialization: The Task Manager loads tasks from an input file and evaluates `robots.txt` for each domain, discarding disallowed URLs.

2. Task Assignment: The Task Manager assigns tasks to Workers based on domain.

3. Task Execution: The Workers complete their tasks within the expected runtime, sending results back to the Task Manager.

4. Result Handling: The Task Manager writes results from the result channel to an output file.

5. Program Termination: When all tasks have been processed, the Task Manager stops all Workers.

### Data Structures

The program mainly uses two data structures: one for input tasks and one for task results. 

Tasks: Tasks are loaded from the input file at the start of the program. 

Results: The results of tasks, which include the URL, status code, any redirects, and time of scan, are returned from the Workers to the Task Manager.

### Session Manager

An optional Session Manager can be included, monitoring the count of ongoing sessions. When the session count exceeds a predefined limit, it can instruct idle Workers to close their sessions.
