## ObserverPolite: A Web Scanner

This scanner program is designed to issue HTTP GET requests to a list of URLs, whilst adhering to the principles of maximizing performance and maintaining "politeness" by ensuring that each domain is accessed only once every 30 seconds.

The program employs Go routines to handle these GET requests, and its architecture comprises two main components: Managers and Workers, with each category having different functionalities.

### Managers

The program will have two managers:

1. **Task Manager:** This manager reads tasks from an input task channel, assigns them to appropriate workers, and retries tasks as necessary. It also collects results from workers and pushes them to an output result channel.

2. **Session Manager:** This manager monitors the count of ongoing sessions, and if the count exceeds a predefined limit, instructs idle workers to close their sessions.

### Workers

Workers are domain-dedicated, each domain maps to a worker. These workers perform two-stage tasks: establish a TLS connection and send a GET request. Both stages are controlled by semaphores to limit concurrency. After completing these stages, the worker returns the result to the Task Manager. The workers also need to communicate with the Session Manager about the status of their TLS connections.

### Program Flow

1. **Initialization:** The program loads tasks into an input task channel. 

2. **Task Assignment:** The Task Manager reads from the task channel and assigns tasks to workers based on domain. 

3. **Task Execution:** The workers then read tasks from their respective channels, establish a TLS connection, and send a GET request. The workers will send a message to the Session Manager upon successful TLS connection establishment. If the Session Manager instructs the worker to close their connection, only idle workers will do so.

4. **Result Handling:** The workers return their task results (success or failure) to the Task Manager. The Task Manager handles successful results by pushing them to a result channel. For failed tasks, the Task Manager will determine if the task should be retried based on the maximum retry limit.

5. **Program Termination:** When the input task channel is empty, the Task Manager informs all Workers and the Session Manager to stop their work.

### Data Structures

Several data structures will be used to manage tasks, workers, sessions and results:

1. **Tasks:** Tasks are loaded into a tasks channel at the beginning of the program.
2. **Results:** Task results are passed from the workers to the Task Manager via a results channel.
3. **Admin Messages:** The Task Manager and the Session Manager send administrative messages to workers via an Admin channel.
4. **Worker Map:** Workers are stored in a map, where the domain serves as the key and the worker as the value.
5. **Sessions:** Session information is communicated between the workers and the Session Manager.

This design ensures optimal use of resources by allowing high concurrency while respecting politeness rules, reusing established connections, and carefully managing session count.
