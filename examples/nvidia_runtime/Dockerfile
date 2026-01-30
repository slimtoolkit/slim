FROM nvcr.io/nvidia/pytorch:25.04-py3

# Add the tests to the entrypoint set. Docker Slim only traces/monitors the processes started by the entrypoint.
RUN echo "pytest /opt/pytorch/pytorch/test/test_cuda.py::TestCuda::test_graph_cudnn_dropout" > /opt/nvidia/entrypoint.d/99-trace.sh
RUN chmod +x /opt/nvidia/entrypoint.d/99-trace.sh

