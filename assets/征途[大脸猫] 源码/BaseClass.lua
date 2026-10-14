require "TSLib";

--调试开关
_DebugSwitch = true;
local DebugMsg = "";

--arr:x,y,color点阵
--jd:精度	--默认80
--ks:保持屏幕开关,当表找色时关闭	--默认开启
function _找色(arr,jd,ks)
	local jd = jd or 80;
	jd = math.floor(0xff*(100-jd)*0.01);
	local ks = true or ks;
	if ks then
		keepScreen(true);
	end
	for var = 1, #arr do
		local lr,lg,lb = getColorRGB(arr[var][1],arr[var][2]);
		local r = math.floor(arr[var][3]/0x10000);
		local g = math.floor(arr[var][3]%0x10000/0x100);
		local b = math.floor(arr[var][3]%0x100);
		if math.abs(lr-r) > jd or math.abs(lg-g) > jd or math.abs(lb-b) > jd then
			keepScreen(false);
			return false;
		end
	end
	if ks then
		keepScreen(false);
	end
	return true;
end

--arr:x,y,color点阵
--pos:点击坐标
--sleep:休眠时间
--jd:精度	--默认80
function _找色点击(arr,pos,sleep,jd)
	local sleep = sleep or 100;
	if _找色(arr,jd) then
		_点击(pos);
		mSleep(sleep);
	end
end

--xycArr:要找的点阵
--jd:精度	--默认80
--return:key or nil
function _表找色(xycArr,jd)
	local jd = jd or 80;
	keepScreen(true);
	for key,value in pairs(xycArr) do
		if _找色(value,jd,false) then
			keepScreen(false);
			return key;
		end
	end
	keepScreen(false);
	return nil;
end

function _表找色点击(xycArr,posArr,sleep,jd)
	local jd = jd or 80;
	local sleep = sleep or 100;
	local result = false;
	keepScreen(true);
	local i = 1;
	if xycArr == nil then
		return;
	end
	for key,value in pairs(xycArr) do
		if _找色(value,jd,false) then
			if type(posArr[key][1]) == "table" then
				for var = 1 , #posArr[key] do
					local temp = posArr[key][var];
					_点击({temp[1],temp[2]});
					mSleep(temp[3]);
				end
			else
				_点击(posArr[key]);
				mSleep(sleep);
			end
			_调试(key);
			result = true;
		end
		i = i+1;
	end
	keepScreen(false);
	return result;
end

--arr:x,y,color点阵
--time:持续时间,默认1000毫秒.参数1为1秒
--jd:精度	--默认80
function _持续找色(xycArr,time,jd)
	keepScreen(true);
	local time = time or 1;
	time = time*1000;
	local time1 = socket.gettime();
	while (true) do
		if _找色(xycArr,jd) then
			keepScreen(false);
			return true;
		end
		time2 = socket.gettime();
		if (time2-time1)*1000 > time then
			keepScreen(false);
			return false;
		end
	end
end

function _持续找不到色(xycArr,time,jd)
	keepScreen(true);
	local time = time or 1;
	time = time*1000;
	local time1 = socket.gettime();
	while (true) do
		if _找色(xycArr,jd) then 
			_调试("找到了,不处理");
		else
			_调试("找不到色");
			keepScreen(false);
			return true;
		end
		time2 = socket.gettime();
		if (time2-time1)*1000 > time then
			keepScreen(false);
			return false;
		end
	end
end

--arr:x,y,color点阵
--time:持续时间,默认1000毫秒.参数1为1秒
--jd:精度	--默认80
function _持续表找色(xycArr,time,jd)
	local time = time or 1;
	time = time*1000;
	local jd = jd or 80;
	local time1 = socket.gettime();
	while (true) do
		local  key = _表找色(xycArr,jd);
		if key ~= nil then 
			_调试("表找色结果:"..key);
			return true,key;
		end
		time2 = socket.gettime();
		if (time2-time1)*1000 > time then
			return false,nil;
		end
	end
end

--arr:要找的点阵
--fw:范围
--jd:精度
function _多点找色(xycArr,fw,jd)
	local jd = jd or 80;
	local x,y = findMultiColorInRegionFuzzyByTable(xycArr,jd,fw[1],fw[2],fw[3],fw[4]);
	if x ~= -1 and y ~= -1 then
		return true,x,y;
	end
	return false,x,y;
end

--arr:要找的点阵
--fw:范围
--sleep:延迟
--jd:精度
function _多点找色点击(xycArr,fw,sleep,jd)
	local sleep = sleep or 100;
	local result,x,y = _多点找色(xycArr,fw,jd);
	if result then
		_点击({x,y});
		mSleep(sleep)
		return true;
	end
	return false;
end

--arr:要找的点阵
--fw:范围
--time:持续时间
--jd:精度
function _持续多点找色(xycArr,fw,time,jd)
	local time = time or 1;
	time = time*1000;
	local time1 = socket.gettime();
	while (true) do
		local result,x,y = _多点找色(xycArr,fw,jd)
		if result then
			return true,x,y;
		end
		time2 = socket.gettime();
		if (time2-time1)*1000 > time then
			return false,0,0;
		end
	end
end

--pos:坐标
--sleep:延迟,默认10~50随机
function _点击(pos,sleep)
	sleep = sleep or 100;
	sleep = math.random(sleep/2,sleep+sleep/2);
	--DebugMsg = DebugMsg.." 点击延迟时间："..sleep;
	touchDown(pos[1],pos[2])
	mSleep(sleep)
	touchUp(pos[1],pos[2])
end

function _运行(AppName,isFront)
	local flag = appIsRunning(AppName); --检测是否在运行
	local isFront = isFront or false;
	if flag  == 0 then                      --如果未运行则启动游戏
		toast("检测到程序未运行,准备运行程序");
		while runApp(AppName) == 1 do
			mSleep(1*1000);
		end
		return true;
	end
	if isFront then
		flag = isFrontApp(AppName); 
		if flag  == 0 then                      --如果未运行则启动游戏
			toast("检测到程序不在前台,准备跳转程序");
			while runApp(AppName) == 1 do
				mSleep(1*1000);
			end
			return false;
		end
	end
	return false;
end

function _播放(audioFilePath,Sleep)
	local Sleep = Sleep or 1;
	Sleep = Sleep*1000;
	setVolumeLevel(1);
	playAudio(audioFilePath); 	
	mSleep(Sleep);           
	setVolumeLevel(0);
	mSleep(1000);   
end

function _调试(showText)
	if _DebugSwitch then
		toast("大脸猫调试:"..DebugMsg..showText,1);
		nLog("大脸猫调试:"..DebugMsg..showText)
		mSleep(1500);
		DebugMsg = "";
		--nLog(showText);
	end
end

function _输入(obj)
	types = getDeviceType(); 
	if types == 0 or types == 1 or types == 2 then
		toast("使用IOS方式输入",1);
		inputText(obj)
	elseif types == 3 or types == 4 then
		toast("使用安卓方式输入",1);
		os.execute("input text \""..obj.."\"")
	end

end

