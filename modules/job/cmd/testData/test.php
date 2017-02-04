<?php
echo "hello world\n";
sleep(1);

file_put_contents("php://stderr", "this is a err \n");