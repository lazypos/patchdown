<!DOCTYPE html>
<html>
<head>
	<link rel="icon" href="/favicon.png" type="image/x-icon">
	<title>远望补丁下载器状态展示</title>
	<style type="text/css">
		body{
			font-size: 14px;
			color: #5c5c5c;
			background-color: #ddd;
		}
		.left-list{
			border: solid 1px #5c5c5c;	
			width: 250px;
			float: left;
			text-align: left;
			padding: 20px;
			background-color: #fff;
		}
		.left-list span{
			color: #c31212;
			float: right;
		}
		.right-list{
			border: solid 1px #5c5c5c;	
			width: 600px;
			margin-left: 10px;
			float: left;
			background-color: #fff;
			font-size: 180px;
		}
		.baselogo{
			font-size: 30px;
			text-align: center;
			padding-bottom: 10px;
		}
		.baseline{
			border-bottom: solid 1px #ccc;
			margin-bottom: 20px;
		}
		.btdownlog{
			padding-left: 10px;
			padding-right: 10px;
			border: solid 1px #3a87d6;
			background-color: #3a87d6;
			color: white;
			font-size: 14px;
			height: 40px;
		}
		.btdownlog:hover{
			background-color: #ccc;
			color: #111;
			cursor:pointer;
		}
		.budingupdate{
			margin-top: 20px;
		}
		.btupdate{
			padding: 5px;
		}
		.zcw11{
			position: relative;
			z-index: 1000
			height:100%;
		}
		.jd{
			font-size: 700px;
			color: #7f93a2;
			position: fixed;
			z-index: 111
			height: 100%;
			width: 50%;
			margin-top: -100px;
			text-align: left;
			background-color: #aaa;
		}
		.jd1{
			font-size: 700px;
			color: #aaa;
			position: fixed;
			z-index: 111
			height: 100%;
			width: 50%;
			margin-top: -100px;
			text-align: right;
			margin-left: 50%;
			background-color: #7f93a2; 
		}
	</style>
</head>
<body style="text-align: center; margin: 20px auto">
	<div class="jd"><b>远</b></div><div class="jd1"><b>望</b></div>
	<div style="display: inline-block;" class="zcw11">
		<div class="left-list">
			<div class="baselogo"><b>服务器运行状态</b></div>
			<div class="baseline"></div>
			<div>程序版本:<span>{{.Version}}</span></div>
			<div>服务启动时间:<span>{{.StartTime}}</span></div></br>

			<div>收到请求总数(次)：<span>{{.TotalQuery}}</span></div>
			<div>下载补丁总数(次)：<span>{{.TotalDown}}</span></div>
			<div>正在下载客户端数(个)：<span>{{.Downing}}</span></div>

			<div>本地补丁个数(个)：<span>{{.LocalCounts}}</span></div></br>
			<!-- <div>补丁校验异常个数(个)：<span>11</span></div></br> -->
			<div>索引更新日期：<span>{{.PatchTime}}</span></div>
			<div>索引下载次数：<span>{{.PatchDowns}}</span></div>
			<!-- <div>最后导入日期：<span>{{.LastImportTime}}</span></div></br> -->
			</br><div class="baselogo"><b>日志下载</b></div>
			<div class="baseline"></div>
			<a href="/logdown?log=run.log" target="_black" download="runlog.zip"><button class="btdownlog">运行日志</button></a>
			<a href="e:\\3.zip" target="_black" download><button class="btdownlog" style="margin-left: 20px; display: none;">下载其他日志</button></a>
			<!-- </br>
			<div class="baselogo budingupdate">补丁升级</div>
			<div class="baseline"></div>
			<label class="btdownlog btupdate" for="xFile">选择补丁包</label>
  			<input type="file" id="xFile" style="position:absolute;clip:rect(0 0 0 0);">
  			<span id="sfile" class="upfile">1111</span></br>
  			<button class="btdownlog" style="margin-top: 20px;">上传补丁包</button> -->
		</div>
		<div class="right-list">维护中</div>
	</div>
<!-- <script src="http://code.jquery.com/jquery-3.2.1.min.js"></script> -->
<script type="text/javascript">
</script>
</body>
</html>