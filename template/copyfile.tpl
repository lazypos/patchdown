<html>
<head>
	<title>补丁导出管理</title>
	<meta http-equiv="pragma" content="no-cache">  
  	<meta http-equiv="cache-control" content="no-cache">   
  	<meta http-equiv="expires" content="0"> 
  	<meta author="2b-zhouchenwei"> 
	<link rel="stylesheet" href="/layui/css/layui.css">
	<style type="text/css">
		body{
			background-color: #ddd;
		}
		.layui-input{
			width: 300px;
		}
		.zcwbody{
			position: relative;
			z-index: 1000
			/*width: 100%;*/
     	  	height: 100%;
			width: 500px;
			border: solid 1px #5c5c5c;
			padding: 20px;
			margin: 200px auto;
			background-color: #fff;
		}
		.layui-form-label{
			width: 180px;
			margin-left: 0px;
			padding-left: 0px;
		}
		.zcwbt{
			margin-top: 0px;
		}
		.zcwlb{
			margin-bottom: 20px;
		}
		.zcwlb span{
			color: #fd2626;
		}
		.layui-upload{
			margin-top: 20px;
			text-align: left;
			margin-left: 20px;
		}
		#sfile{
			font-size: 20px;
		}
		.zcwline{
			margin-top: 10px;
			margin-bottom: 10px;
		}
		.zcwtt{
			border-bottom: solid 1px #ccc;
			padding-bottom: 10px;
			margin-bottom: 20px;
			font-size: 30px;
			color: #5a5a5a;
		}
		.zcwtt span{
			font-size: 16px;
			margin-left: 20px;
		}
		.zcwpro{
			margin-top: 15px;
		}
		.jd{
			font-size: 700px;
			color: #7f93a2;
			position: fixed;
			z-index: 111
			height: 100%;
			width: 50%;
			margin-top: -300px;
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
			margin-top: -300px;
			text-align: right;
			margin-left: 50%;
			background-color: #7f93a2; 
		}
	</style>
</head>
<body style="text-align: center;">
	<div class="jd"><b>远</b></div><div class="jd1"><b>望</b></div>
	<div class="zcwbody">
	<div class="zcwtt"><b>远望补丁导出管理</b><span>ver: {{.Version}}</span></div>
	<div class="zcwlb"><label>上一次补丁导出时间：</label><span>{{.LastTime}}</span></div>
	<div class="layui-inline">
      <label class="layui-form-label">自定义补丁导出时间范围：</label>
      <div class="layui-input-inline">
        <input type="text" class="layui-input" id="rangetime" placeholder="点击选择时间范围" lay-key="6">
      </div>
    </div>

    <div class="layui-inline zcwline">
      <label class="layui-form-label">导出补丁位置：</label>
      <input type="text" class="layui-input" id="dstpos" placeholder="d:\ -----盘符请带冒号 :" >
    </div>
    <div class="zcwbt">
    	<button class="layui-btn dc" onclick="CopyPatch(1)">全部导出</button>
    	<button class="layui-btn dc" onclick="CopyPatch(2)">增量导出</button>
    	<button class="layui-btn dc" onclick="CopyPatch(3)">自定义时间导出</button>
    	<button class="layui-btn  layui-btn-warm qx" onclick="CancelPatch()" style="display: none;">（补丁正在导出中...）点击取消导出</button>
    </div>
    <div class="layui-progress layui-progress-big zcwpro" lay-filter="proc" lay-showPercent="yes" style="display: none;">
  		<div class="layui-progress-bar layui-bg-red" lay-percent="0%" id="process"></div>
	</div>
	</div>
</body>
<script src="/layui/layui.js"></script>
<script src="/layui/jquery-1.7.2.min.js"></script>
<script src="/layui/jquery.timers.js"></script>
<script type="text/javascript">
	layui.use('layer', function(){});
	layui.use('element', function(){
	  var element = layui.element;
	});

	layui.use('laydate', function(){
  		var laydate = layui.laydate;
		laydate.render({
		    elem: '#rangetime'
		    ,type: 'datetime'
		    ,range: true
		});
	});

	function QueryProcess(){
		url = "/process?" + Date.parse(new Date());
		$.get(url, function(data, status){ 
			if (data.error != "") {
				$('body').stopTime();
				$('.dc').show();
			    $('.qx').hide();
			    $('.zcwpro').hide();
				layui.use('layer', function(){
					layui.layer.alert(data.error, {icon: 5});
				});
			}else{
				layui.element.progress('proc', data.message);
				if (data.message == "100%") {
					$('body').stopTime();
					layui.use('layer', function(){
						layui.layer.alert(data.info, {icon: 6});
					}); 
					$('.qx').hide();
				}
			}
		});
	}

	function CopyPatch(type){
		var dst = $('#dstpos').val()
		var times = $('#rangetime').val()
		if (dst.length == 0) {
			layui.use('layer', function(){
				layui.layer.alert('目的文件夹不能为空！', {icon: 5});
			});
			return false;
		}
		if (type == 3 && times.length==0) {
			layui.use('layer', function(){
				layui.layer.alert('请选择导出补丁的时间范围！', {icon: 5});
			});
			return false;
		}
		layui.use('layer', function(){
	      	time: 0
		    layui.layer.alert('确定要导出吗？', {
			    icon: 6
			    ,btn: ['是','否']
			    ,yes:function(index){
			    	layer.close(index);
			    	$('.dc').hide();
			    	$('.qx').show();
			    	$('.zcwpro').show();

			    	//发送请求
			    	url = "/copy?type=" +type+"&dest="+dst +"&time="+times+"&d="+Date.parse(new Date());
					$.get(url, function(data, status){ 
						if (data.message == "") {
							$('body').everyTime('2s', QueryProcess);
						}else{
							layui.layer.alert(data.message, {icon: 5});
				    		$('.dc').show();
					    	$('.qx').hide();
					    	$('.zcwpro').hide();
						}
					});
			    }
			});
		});
	}

	function CancelPatch(type){
		layui.use('layer', function(){
	      	time: 0
		    layui.layer.alert('确定要取消吗？', {
			    icon: 6
			    ,btn: ['是','否']
			    ,yes:function(index){
			    	layer.close(index);
			    	$('.dc').show();
			    	$('.qx').hide();
			    	$('.zcwpro').hide();

			    	$('body').stopTime();
				}
			});
		});
	}
</script>
</html>