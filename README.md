# go-observability-bench

## Terminology

- `Workload`: A Go function performing a small task (< 100ms) like parsing a big blob of JSON or serving an http request.
- `Run`: Running a Go program that executes a given `Workload` for a certain duration in a loop while running one or more `Profilers` in parallel.
- `Op`: A single invocation of the `Workload` function. A `Run` executes many `Ops`.
- `Job`: A named set of `Run` configurations, including which profilers to enable during the run. Usually there is a baseline job that runs several `Workloads` without profiling, as well as jobs that run the same workloads with various profilers enabled.
- `Config`: A set of `Jobs` to execute.
- `Profiler`: A tool that captures performance data during a `Run`.
