import unittest
import redis
import subprocess
import time
import os


class TestRedisKey(unittest.TestCase):

    r = redis.StrictRedis(host='localhost', port=6379, db=0)

    def test01_flush_db(self):
        self.r.flushdb()

    def test02_execute_test(self):
        fdnull = open(os.devnull, 'w')
        cmd = ["./test.sh"]
        subprocess.call(cmd, stdout=fdnull, stderr=subprocess.STDOUT)
        time.sleep(4)

    def test05_check_the_value(self):
        val = self.r.get("key1")
        if val:
            self.assertEqual(val, "value1")
        else:
            self.fail("key not found")

if __name__ == '__main__':
    unittest.main()
