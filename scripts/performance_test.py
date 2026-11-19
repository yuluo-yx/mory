"""Verify native benchmark failures clean up resources before propagating."""
from pathlib import Path
import tempfile
import unittest
from unittest.mock import Mock, patch

from performance import NativeRun


class NativeRunCleanupTests(unittest.TestCase):
    def test_constructor_failures_join_process_and_close_resources(self):
        for phase in ("connect", "capture"):
            with self.subTest(phase=phase), tempfile.TemporaryDirectory() as folder:
                process = Mock(pid=12345)
                process.poll.return_value = None
                channel = Mock()
                if phase == "connect":
                    channel.connect.side_effect = OSError("AF_UNIX path too long")
                else:
                    channel.makefile.return_value.readline.side_effect = TimeoutError("capture timed out")
                monitor = Mock()
                run = NativeRun.__new__(NativeRun)
                with patch("performance.subprocess.Popen", return_value=process), \
                     patch("performance.socket.socket", return_value=channel), \
                     patch("performance.threading.Thread", return_value=monitor):
                    with self.assertRaises((OSError, RuntimeError)):
                        run.__init__(Path("unused-binary"), Path(folder), 1000, 700, .01)
                process.wait.assert_called_once_with(timeout=10)
                monitor.join.assert_called_once_with(timeout=2)
                channel.close.assert_called_once()
                self.assertTrue(run.stop.is_set())
                self.assertTrue(run.errors.closed)
                if phase == "capture":
                    channel.makefile.return_value.close.assert_called_once()


if __name__ == "__main__":
    unittest.main()
