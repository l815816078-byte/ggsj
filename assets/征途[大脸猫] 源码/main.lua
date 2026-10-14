require("TSLib")
require("BaseClass")
require("PointLib")

local AppName = "com.tencent.zhengtuiphone";
local width,height = getScreenSize();
local ScripteModel = "";

function _UserSetting()

	UINew(1,"【大脸猫科技】征途工作室脚本","运行","退出","dlm_zt_1",1,30,height,width,"255,255,255","255,255,255","","number",1)

	UILabel(1,"调试【开启将降低运行效率】",18,"center","255,0,0")
	UIRadio(1,"ts","开启,关闭","0")

	UILabel(1,"模式",18,"center","255,0,0")
	UIRadio(1,"ms","起号,出金,拉骆驼","0")

	UIShow();

	if ts == "开启" then
		_DebugSwitch = true;
	else
		_DebugSwitch = false;
	end
	ScripteModel = ms;
end

--初始化屏幕
function _InitScreenSize()
	if width == 640 and height == 1136 then
		toast("屏幕初始化成功",1);
		_Feature();
	else
		dialog("您的手机型号暂未支持", 0)
		lua_exit();
	end
end

--初始化
function _Init()
	init(AppName,1);
	_InitScreenSize();
	_UserSetting();
	_TaskDate();
	_GameData();
end

function _Test()
	if _找色(Task_t.种植养殖) then
		_调试("种植养殖");
	end
end

function _GameData()
	游戏参数 = 
	{
		["当前区"] = 1,
		["当前角色"] = 1,
	}
end

--游戏主流程
function _GameFlow()
	while (true) do
		--_Test();
		if _运行(AppName) then
			mSleep(10000);
			_点击({1,1});
		end

		_RoleFlow();
		_GuideTask();
		_GameWindow();
		_Task();
		_Activity();

		_表找色点击(TopOneWindow_t,TopOneWindow_d,75);
		_表找色点击(Default_t,Default_d);

		if ScripteModel == "起号" then
			if _找色(Point_t.等级40) or _找色(Point_t.等级50) then
				_调试("练级等级已达标,进入换角色流程");
				if 游戏参数.当前角色 >= 5 then
					游戏参数.当前区 = 游戏参数.当前区 + 1;
				else
					游戏参数.当前角色 = 游戏参数.当前角色 + 1;
				end
				closeApp(AppName);
				_TaskDate();
			end
		elseif ScripteModel == "出金" then
			if 任务参数.种植养殖 == 任务类型.已完成 and 任务参数.无尽探索 == 任务类型.已完成  then
				if 任务参数.代练_刺探敌情 == true and 任务参数.代练_王城运镖 == true and 任务参数.代练_搬砖 == true then
					if 游戏参数.当前角色 >= 5 then
						游戏参数.当前区 = 游戏参数.当前区 + 1;
					else
						游戏参数.当前角色 = 游戏参数.当前角色 + 1;
					end
					closeApp(AppName);
					_TaskDate();
				else 
					_Whereas();
				end
			end
		elseif  ScripteModel == "拉骆驼" then
			if 任务参数.代练_刺探敌情 == true and 任务参数.代练_王城运镖 == true and 任务参数.代练_搬砖 == true then
				任务参数["当前任务"] = "骆驼商队";
				if 任务参数.骆驼商队 == 任务类型.已完成 then
					if 游戏参数.当前角色 >= 5 then
						游戏参数.当前区 = 游戏参数.当前区 + 1;
					else
						游戏参数.当前角色 = 游戏参数.当前角色 + 1;
					end
					closeApp(AppName);
					_TaskDate();
				end
			else
				_Whereas();
			end
		end
	end
end

function _Whereas()
	if _找色(Task_t.任务收起1) == true or _找色(Task_t.任务收起2) == true then
		for var = 1,#Point_d.代练 do
			_点击({Point_d.代练[var][1],Point_d.代练[var][2]});
			_调试("代练"..var);
			mSleep(Point_d.代练[var][3]);
		end
	end
	if _找色(Point_t.替身宝宝面板) == false then
		return;
	else
		_调试("在辅助代练界面了");
	end

	local result,x,y;
	for var = 1,10 do
		if 任务参数.代练_刺探敌情 == false then
			result,x,y = _多点找色(Point_t.代练_刺探敌情,Scope.代练,70);
			if result then
				_调试("代练_刺探敌情");
				_点击({x+Const.代练偏移[1],y+Const.代练偏移[2]});
				mSleep(1000);
				任务参数.代练_刺探敌情 = true;
			end
		end
		if _找色(Point_t.代练确定) then
			_点击(Point_d.代练确定);
			mSleep(1000);
		end
		if 任务参数.代练_搬砖 == false then
			result,x,y = _多点找色(Point_t.代练_搬砖,Scope.代练,70);
			if result then
				_调试("代练_搬砖");
				_点击({x+Const.代练偏移[1],y+Const.代练偏移[2]});
				mSleep(1000);
				任务参数.代练_搬砖 = true;
			end
		end
		if _找色(Point_t.代练确定) then
			_点击(Point_d.代练确定);
			mSleep(1000);
		end
		if 任务参数.代练_王城运镖 == false then
			result,x,y = _多点找色(Point_t.代练_王城运镖,Scope.代练,70);
			if result then
				_调试("代练_王城运镖");
				_点击({x+Const.代练偏移[1],y+Const.代练偏移[2]});
				mSleep(1000);
				任务参数.代练_王城运镖 = true;
			end
		end
		if _找色(Point_t.代练确定) then
			_点击(Point_d.代练确定);
			mSleep(1000);
		end
		moveTo(Move.代练[1],Move.代练[2],Move.代练[3],Move.代练[4],5,50);
		mSleep(1000);
	end
	_点击(Point_d.替身宝宝面板关闭);
end

function _GameWindow()
	local windowIndex = 1;
	while _找色(Point_t.任务弹窗) do
		_调试("任务弹窗"..windowIndex);
		_点击(Point_d.任务弹窗[windowIndex]);
		mSleep(100);
		windowIndex = windowIndex + 1;
		if windowIndex > #Point_d.任务弹窗 then
			break;
		end
	end
end

function _RoleFlow()
	if _找色(Point_t.创建角色) then
		_点击(Point_d.随机名字);
		mSleep(1000);
		_点击(Point_d.输入名字);
		_输入("喵");
		mSleep(1000);
		_点击(Point_d.输入完成);
		mSleep(1000);
		_点击(Point_d.创建角色);
	end
	if _找色(Point_t.踏上征程) then
		_点击(Point_d["角色"..游戏参数.当前角色]);
		mSleep(1000);
		_点击(Point_d.踏上征程);
	end
end

function _GuideTask()
	if _找色(Point_t.进度条) then
		_调试("进度条");
		mSleep(1000);
		return;
	end

	if _找色(Point_t.请按指引操作) then
		_调试("请按指引操作");
		if _持续找色(Point_t.日常引导,2) then
			_调试("日常引导");
			_点击(Point_d.日常引导);
			mSleep(2000);
			_点击(Point_d.引导_种植);
			mSleep(2000);
		end
		if _持续找色(Point_t.提升引导,2) then
			_调试("提升引导");
			_点击(Point_d.提升引导);
			mSleep(2000);
		end
		if _持续找色(Point_t.背包引导,2) then
			_调试("背包引导");
			_点击(Point_d.背包引导);
			mSleep(3000);
			_点击(Point_d.背包锻造);
			mSleep(3000);
			_点击(Point_d.锻造武器);
			mSleep(3000);
			_点击(Point_d.选择武器类型);
			mSleep(3000);
			_点击(Point_d.选择锻造武器);
			mSleep(3000);
			_点击(Point_d.选择锻造材料);
			mSleep(3000);
			_点击(Point_d.购买材料);
			mSleep(3000);
			_点击(Point_d.打造装备);
			mSleep(3000);
			_点击(Point_d.关闭打造);
			mSleep(3000);
		end
	end
end

function _TaskDate()
	任务类型 = 
	{
		["未开始"]="未开始",
		["任务中"]="任务中",
		["已完成"]="已完成",
	}
	任务参数 = 
	{
		["当前任务"] = "主线",
		["最大适配任务次数"] = 10,
		["当前适配任务次数"] = 0,

		["种植养殖"] = 任务类型.未开始;
		["王城运镖"] = 任务类型.未开始;
		["无尽探索"] = 任务类型.未开始;

		["骆驼商队"] = 任务类型.未开始;

		["代练_刺探敌情"] = false;
		["代练_王城运镖"] = false;
		    ["代练_搬砖"] = false;
	}
	if ScripteModel == "出金" then
		任务参数["当前任务"] = "无尽探索";
	elseif  ScripteModel == "拉骆驼" then
		任务参数["当前任务"] = "";
	end
end

--切换任务
function _TaskFlow(task)
	任务参数.当前适配任务次数 = 0;
	if task == nil then
		if ScripteModel == "起号" then
			if 任务参数.当前任务 == "主线" then
				任务参数.当前任务 = "种植养殖";
				return;
			end
			if 任务参数.当前任务 == "种植养殖" then
				任务参数.当前任务 = "王城运镖";
				return;
			end
			if 任务参数.当前任务 == "王城运镖" then
				任务参数.当前任务 = "无尽探索";
				return;
			end
			if 任务参数.当前任务 == "无尽探索" then
				任务参数.当前任务 = "主线";
				return;
			end
		elseif ScripteModel == "出金" then
			if 任务参数.当前任务 == "无尽探索" then
				任务参数.当前任务 = "种植养殖";
				return;
			end
			if 任务参数.当前任务 == "种植养殖" then
				任务参数.当前任务 = "无尽探索";
				return;
			end
		end
	else
		任务参数.当前任务 = task;
	end
end

function _Task()
	if _找色(Task_t.任务收起1) == false and _找色(Task_t.任务收起2) == false then
		return;
	end

	if 任务参数.当前适配任务次数 >= 任务参数.最大适配任务次数 then
		_TaskFlow();
	end

	任务参数.当前适配任务次数 = 任务参数.当前适配任务次数 + 1;

	local resutl,x,y ;
	if 任务参数.当前任务 == "主线" then
		result,x,y = _多点找色(Task_t.主线,Scope.任务,70);
		if result then
			_调试("主线");
			_点击({x,y});
			mSleep(1000);
			任务参数.当前适配任务次数 = 0;
		end
	elseif 任务参数.当前任务 == "王城运镖"  then
		result,x,y = _多点找色(Task_t.押运镖车1,Scope.任务,70);
		if result then
			_调试("押运镖车1");
			_点击({x,y});
			mSleep(1000);
			任务参数.当前适配任务次数 = 0;
		else
			result,x,y = _多点找色(Task_t.押运镖车2,Scope.任务,70);
			if result then
				_调试("押运镖车2");
				_点击({x,y});
				mSleep(1000);
				任务参数.当前适配任务次数 = 0;
			end
		end
	elseif 任务参数.当前任务 == "种植养殖"  then
		resutl,x,y = _多点找色(Task_t.种植养殖,Scope.任务,80);
		if resutl then
			_调试("种植养殖");
			_点击({x,y});
			mSleep(1000);
			任务参数.当前适配任务次数 = 0;
			任务参数.种植养殖 = 任务类型.任务中;
		else
			_点击(Task_d.日常活动);
		end
	elseif 任务参数.当前任务 == "无尽探索"  then
		resutl,x,y = _多点找色(Task_t.无尽探索,Scope.任务,80);
		if resutl then
			_调试("无尽探索");
			_点击({x,y});
			mSleep(1000);
			任务参数.当前适配任务次数 = 0;
			任务参数.无尽探索 = 任务类型.任务中;
		else
			_点击(Task_d.日常活动);
		end
	elseif 任务参数.当前任务 == "骆驼商队"  then
		resutl,x,y = _多点找色(Task_t.骆驼商队,Scope.任务,80);
		if resutl then
			_调试("骆驼商队");
			_点击({x,y});
			mSleep(1000);
			任务参数.当前适配任务次数 = 0;
			任务参数.骆驼商队 = 任务类型.任务中;
		else
			_点击(Task_d.日常活动);
		end
	end
	if 任务参数.当前适配任务次数 == 1 or 任务参数.当前适配任务次数 == 10 then
		_调试("当前适配任务:"..任务参数.当前任务..",适配次数:"..任务参数.当前适配任务次数);
	end
end

function _Activity()
	if _找色(Point_t.日常面板) then
		if 任务参数.当前任务 == "主线" or 任务参数.当前任务 == "种植养殖"  then
			resutl,x,y = _多点找色(Point_t.种植养殖,Scope.日常,80);
			if resutl then
				_调试("种植养殖");
				_点击({x+Const.活动偏移[1],y+Const.活动偏移[2]});
				mSleep(10000);
				_TaskFlow("种植养殖");
			else
				任务参数.种植养殖 = 任务类型.已完成;
				_TaskFlow();
				_调试("种植养殖 任务完成");
			end
		end
		if 任务参数.当前任务 == "主线" or 任务参数.当前任务 == "王城运镖" then
			resutl,x,y = _多点找色(Point_t.王城运镖1,Scope.日常,80);
			if resutl then
				_调试("王城运镖");
				_点击({x+Const.活动偏移[1],y+Const.活动偏移[2]});
				mSleep(10000);
				_TaskFlow("王城运镖");
			else
				resutl,x,y = _多点找色(Point_t.王城运镖2,Scope.日常,80);
				if resutl then
					_调试("王城运镖");
					_点击({x+Const.活动偏移[1],y+Const.活动偏移[2]});
					mSleep(10000);
					_TaskFlow("王城运镖");
				else
					任务参数.王城运镖 = 任务类型.已完成;
					_TaskFlow();
					_调试("王城运镖 任务完成");
				end
			end
		end
		if 任务参数.当前任务 == "无尽探索" then
			resutl,x,y = _多点找色(Point_t.无尽探索,Scope.日常,80);
			if resutl then
				_调试("无尽探索");
				_点击({x+Const.活动偏移[1],y+Const.活动偏移[2]});
				mSleep(10000);
			else
				任务参数.无尽探索 = 任务类型.已完成;
				_TaskFlow();
				_调试("无尽探索 任务完成");
			end
		end
		if 任务参数.当前任务 == "骆驼商队" then
			resutl,x,y = _多点找色(Point_t.骆驼商队,Scope.日常,80);
			if resutl then
				_调试("骆驼商队");
				_点击({x+Const.活动偏移[1],y+Const.活动偏移[2]});
				mSleep(10000);
			else
				任务参数.骆驼商队 = 任务类型.已完成;
				_TaskFlow();
				_调试("骆驼商队 任务完成");
			end
		end
		if _找色(Point_t.日常面板) then
			_点击(Point_d.日常面板);
			mSleep(1000);
		end
	end
end

_Init();
_GameFlow();