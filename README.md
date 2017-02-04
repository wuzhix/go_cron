# 计划任务调度系统

## 依赖环境
	
	1、mongoDB



## 部署步骤

### centos6下的部署
1、下载调度程序到相关目录，这里默认使用 /data/go/src/jcron  目录下
2、修改程序数据链接和ip等配置信息
3、复制scripts/init.d目录下的脚本文件至系统的/etc/init.d/目录下，并对脚本进行修改，只需要修改程序的执行路径，也就是步骤1的程序路径
4、启动服务  

```
	chkconfig jcron_modules on
	service jcron_modules start
	chkconfig jcron_website on
	service jcron_website start
```

### centos7下的部署
1、下载调度程序到相关目录，这里默认使用 /data/go/src/jcron  目录下
2、修改程序数据链接和ip等配置信息
3、复制scripts/systemctl目录下的脚本文件至系统的/etc/systemd/system/目录下，并对脚本进行修改，只需要修改程序的执行路径，也就是步骤1的程序路径
4、启动服务  

```
	systemctl enable jcron_modules
	systemctl start jcron_modules
	systemctl enable jcron_website
	systemctl start jcron_website
```
