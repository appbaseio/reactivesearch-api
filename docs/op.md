# Operation

Operation delineates the kind of operation a request intends to make. The operation of the request is identified before the request is served. The classification of the request operation depends on the use-case and the implementation of the plugin. Operation is currently classified into three kinds:

- `Read`: operation permits read requests exclusively.
- `Write`: operation permits write requests exclusively.
- `Delete`: operation permits delete requests exclusively.

In order to allow a user or permission to make requests that involve modifying the data, a combination of the above operations would be required. For example: `["read", "write"]` operation would allow a user or permission to perform both read and write requests but would forbid making delete requests.